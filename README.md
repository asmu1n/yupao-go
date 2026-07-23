# Yupao Go

伙伴匹配系统后端（Go）。本文面向团队协作：说明**目录如何划分**、**依赖怎么走**、**新人如何接入开发**。

---

## 1. 设计目标（为什么这样拆）

| 目标         | 做法                                                            |
| ------------ | --------------------------------------------------------------- |
| 业务可扩展   | 业务只放在 `internal/module/<name>/`，不在 `internal/` 顶层平铺 |
| 依赖清晰     | 业务依赖 `port` 接口，不直接依赖 Redis/DB 实现细节              |
| 改动半径可控 | 用户相关能力集中在 `module/user`（含 HTTP Handler、Repo、预热） |
| 入口干净     | `cmd/*` 只做装配与生命周期，不写业务规则                        |
| 公共代码克制 | `pkg` 只放与具体业务无关的工具；业务逻辑不要下沉                |

一句话：**业务进 module，协议进 httpapi，能力抽象进 port，技术细节进 infra，公共工具进 pkg，进程在 cmd 拧在一起。**

---

## 2. 目录结构

```text
yupao-go/
├── cmd/
│   ├── server/          # HTTP API 入口（装配 DB/Redis/Service/路由/定时任务）
│   └── seed/            # 生成测试用户 SQL 等辅助命令
├── docs/
│   ├── api/swagger/     # OpenAPI 生成物（Swagger UI 读这里）
│   └── REDIS_CACHE.md   # Redis / 缓存总览
├── ent/
│   └── schema/          # 手写 ent schema（改表结构只动这里，再 generate）
├── internal/
│   ├── module/          # ★ 业务模块（按领域垂直切片）
│   │   ├── user/        # 用户：注册登录、标签搜索、匹配与缓存预热
│   │   │   ├── http/    # Handler
│   │   │   ├── repo/
│   │   │   └── …        # service / match / CACHE.md
│   │   └── team/        # 队伍：创建/加入/退出/解散/列表
│   │       ├── http/
│   │       ├── repo/
│   │       └── …
│   ├── httpapi/         # 路由注册、鉴权中间件
│   ├── port/            # 跨模块技术端口（Cache、Locker）
│   ├── infra/           # 基础设施实现（DB、Redis、缓存、锁、定时器）
│   ├── pkg/             # 公共库（logger、分页、统一响应、基础类型）
│   └── config/          # 环境变量加载
├── docker-compose.yml       # 公共底座：postgres / redis / app
├── docker-compose.dev.yml   # 开发叠加：暴露依赖端口，默认不起 app
├── Dockerfile
├── .env.example
├── .github/workflows/       # CI（test）+ Image（GHCR）
└── test/                # 集成/连通类测试（可选）
```

### 各层职责

| 路径                | 职责                                  | 典型改动                       |
| ------------------- | ------------------------------------- | ------------------------------ |
| `cmd/server`        | 组装依赖、启停 HTTP/cron、`logger.Init` | 新模块注入、新定时任务       |
| `internal/module/*` | 领域模型、用例、该业务的 API 与持久化 | **日常业务开发主战场**         |
| `internal/httpapi`  | 挂路由、全局鉴权                      | 注册新 module 的路由           |
| `internal/port`     | Cache / Locker 等抽象                 | 新增跨模块技术能力时扩接口     |
| `internal/infra`    | 上述端口的 Redis/DB/cron 实现         | 换客户端、调连接与中间件配置   |
| `internal/pkg`      | logger、分页、错误码与响应体、通用枚举 | 真正跨业务复用时才加          |
| `ent/schema`        | 表结构与字段约束                      | 加字段、改索引后 `go generate` |

更细的模块约定见：[`internal/module/README.md`](internal/module/README.md)  
公共库约定见：[`internal/pkg/README.md`](internal/pkg/README.md)  
结构化日志见：[`internal/pkg/logger/README.md`](internal/pkg/logger/README.md)  
匹配缓存细节见：[`internal/module/user/CACHE.md`](internal/module/user/CACHE.md)  
队伍事务与行锁见：[`internal/module/team/TX_LOCK.md`](internal/module/team/TX_LOCK.md)

---

## 3. 依赖方向（必读）

```text
cmd/server
    │
    ▼
 httpapi  ──────────────────►  module/*/http
    │                               │
    │                               ▼
    │                          module/* (Service)
    │                               │
    │                    ┌──────────┼──────────┐
    │                    ▼          ▼          ▼
    │                  port        pkg     （其他 module 的 Service）
    │                    ▲
    │                    │ 实现
    └──────────────►  infra
```

**规则：**

1. `module` **不要** import `infra` 具体实现，只依赖 `port`（及 `pkg`）。
2. `infra` 实现 `port`；可依赖 `ent`、Redis 客户端等。
3. 跨业务模块：只调用对方 **Service 公开方法**，不要直接依赖对方 `repo`。
4. 避免循环依赖：鉴权在 `httpapi/middleware`，供 `module/*/http` 使用；路由装配在 `httpapi` 引用各模块 Handler。

---

## 4. 请求怎么走（帮助建立心智模型）

以「匹配相似用户」为例：

```text
GET /api/user/match
  → httpapi 路由 + AuthRequired（session）
  → module/user/http.Handler
  → user.Service.MatchUsers
       → port.Cache（TryFetch / Once）
       → miss：活跃候选池 + 标签编辑距离 + Top-K
  → infra/cache 实际读写 Redis（及进程内 L1）
```

定时预热在 `cmd/server` 用 `infra/scheduler` 触发，逻辑仍在 `user.Service.WarmUpMatchUsers`（与在线匹配共用候选池定义，见 `CACHE.md`）。

---

## 5. 本地开发

### 5.1 依赖

- Go（版本见 `go.mod`）
- Docker（可选，用于 Postgres / Redis）

### 5.2 基础设施

```bash
# 准备环境变量（契约见 .env.example）
cp .env.example .env

# 仅启动 Postgres + Redis（本机 go run 使用）
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d

# 可选：容器内全栈（app 也进 compose，需 --profile full）
# docker compose -f docker-compose.yml -f docker-compose.dev.yml --profile full up -d --build
```

本机跑 API 时 `.env` 使用 `DB_HOST=localhost`、`REDIS_HOST=localhost`，端口与 `*_HOST_PORT` 映射一致。  
compose 内的 app 由 `docker-compose.yml` 注入 `DB_HOST=postgres` / `REDIS_HOST=redis` 及容器内端口。

### 5.3 运行 API

```bash
# 可选日志相关环境变量：
#   LOG_LEVEL=debug|info|warn|error
#   LOG_FORMAT=text|json
#   SERVICE_NAME=yupao-api
#   ENV=prod   # 未设 LOG_FORMAT 时 prod/production 默认 json
go run ./cmd/server
# 默认 :8080
# 健康检查：http://localhost:8080/health
# Swagger UI：http://localhost:8080/swagger/index.html
# 生成物目录：docs/api/swagger（import: yupao-go/docs/api/swagger）
```

访问日志目前来自 **Gin 默认 Logger**（`gin.Default`）；业务 / 任务 / 审计使用 `internal/pkg/logger` 结构化输出（stderr）。详见 [logger README](internal/pkg/logger/README.md)。

### 5.4 常用命令

```bash
go build ./...
go test ./...   # test/ 包需要本机 Postgres（见 5.2）

# 修改 ent/schema 后重新生成
go generate ./ent
```

### 5.5 测试数据（可选）

```bash
go run ./cmd/seed -h   # 查看参数；可生成批量用户 SQL
```

### 5.6 CI / 镜像（GitHub Actions）

| Workflow | 触发 | 做什么 |
| -------- | ---- | ------ |
| [`.github/workflows/ci.yml`](.github/workflows/ci.yml) | PR、`main` push | 起 Postgres/Redis → `go test ./...` → `go build ./cmd/server` |
| [`.github/workflows/image.yml`](.github/workflows/image.yml) | `main` push、`v*` tag | 先同样跑测试，再 build/push 镜像到 GHCR |

镜像名（小写）：

```text
ghcr.io/<owner>/<repo>:<git-sha-short>
ghcr.io/<owner>/<repo>:latest          # 仅 default branch
ghcr.io/<owner>/<repo>:v1.2.3          # 仅 semver tag（如 v1.2.3）
```

首次从私有仓库拉镜像需登录：

```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin
docker pull ghcr.io/<owner>/<repo>:latest
```

Package 可见性在 GitHub → Packages 中调整。建议在仓库 Settings → Branches 为 `main` 开启 **Require status checks**（勾选 `CI / Test`），避免未测代码直接进主分支。

---

## 6. 如何接入一个新功能

### A. 在已有模块内加接口（例如 user）

1. `module/user`：模型 / Service 方法 / 如需则扩展 `Repository` 接口。  
2. `module/user/repo`：实现仓储方法。  
3. `module/user/http`：Handler + Swagger 注释。  
4. `httpapi/user.go`：注册路由（注意是否需 `AuthRequired`）。  
5. 如改表：`ent/schema` → `go generate ./ent` → 迁移（当前由服务启动 migrate，以项目现状为准）。

### B. 新增业务模块（例如 team）

1. 创建 `internal/module/team/`（建议同样拆 `http/`、`repo/`）。  
2. 在 `httpapi` 增加路由注册。  
3. 在 `cmd/server` 构造 Service 并注入。  
4. 不要把 team 的业务逻辑写进 `user` 或 `pkg`。

### C. 需要缓存 / 分布式锁

- 业务侧使用 `port.Cache` / `port.Locker`。  
- 实现已在 `infra/cache`、`infra/lock`；一般只需在 `NewService` 注入，无需业务包 import infra。

---

## 7. 放哪里？快速判定

| 你要加的内容                         | 放哪里                        |
| ------------------------------------ | ----------------------------- |
| 某业务的用例、模型、该业务 API       | `module/<name>/`              |
| 全局路由挂载、登录态中间件           | `httpapi`                     |
| 「我需要锁/缓存，不关心 Redis」      | `port` 接口 + `infra` 实现    |
| 分页、统一 JSON 响应、通用 Gender 等 | `pkg`                         |
| 结构化业务/任务/审计日志             | `pkg/logger`（Service/Job 打点） |
| 仅某一业务用的算法                   | 留在该 `module`，不要进 `pkg` |
| 表结构                               | `ent/schema`                  |
| 进程启动参数、组装顺序               | `cmd/*`                       |

---

## 8. 协作约定（简）

1. **优先在对应 module 内闭环**；跨模块先谈 Service 接口，避免双向 import 实现细节。  
2. **改缓存语义时** 保证在线路径与预热路径候选集一致（参见 `module/user/CACHE.md`）。  
3. **生成代码**（`ent/*` 非 schema、`docs/api/swagger`）不要手改业务逻辑；改源再生成。  
4. **PR 粒度**：一个业务能力尽量带齐 service + handler + repo（及必要测试），便于评审。  
5. **命名**：新 module 用小写业务名；HTTP 子包可用 `userhttp` 这类包名，避免与 `net/http` 冲突。  
6. **日志**：新写路径用 `logger.Module` + `purpose` + 稳定 `event`；可预期 `BizError` 不打 Error；系统错误交给 `RespondError` 边界记一次（见 [logger README](internal/pkg/logger/README.md)）。

---

## 9. 相关文档索引

| 文档                                                           | 内容                                |
| -------------------------------------------------------------- | ----------------------------------- |
| [internal/module/README.md](internal/module/README.md)         | 业务模块目录约定                    |
| [internal/pkg/README.md](internal/pkg/README.md)               | 公共库边界                          |
| [internal/pkg/logger/README.md](internal/pkg/logger/README.md) | **结构化日志约定与 event 表**       |
| [docs/REDIS_CACHE.md](docs/REDIS_CACHE.md)                     | **项目级 Redis / 缓存策略**（总览） |
| [internal/module/user/CACHE.md](internal/module/user/CACHE.md) | 匹配查询缓存细节与流程图            |
| [internal/module/team/TX_LOCK.md](internal/module/team/TX_LOCK.md) | **队伍事务 / team 行锁与 Join Redis 锁分工** |

有疑问时：先看依赖图（第 3 节）和「放哪里」（第 7 节），再在对应目录下搜索现有 `user` 实现作为模板。
