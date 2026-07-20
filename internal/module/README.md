# 业务模块（module）

所有业务能力放在本目录下，避免在 `internal/` 顶层平铺过多包。

## 约定

```text
internal/module/<name>/
  model.go / service.go / repository.go   # 领域与用例
  cache_task.go …                         # 可选：任务、匹配等
  http/                                   # 传输层 Handler（package <name>http）
  repo/                                   # 持久化实现（package repo）
```

### 已有模块

| 模块 | 说明 |
|------|------|
| `user` | 注册登录、标签搜索、匹配与缓存预热 |
| `team` | 队伍创建/加入/退出/解散、列表与分页（对齐 Java 参考） |

新增业务时：

1. 新建 `internal/module/<name>/`，按上表拆分。
2. 在 `internal/httpapi` 注册路由。
3. 在 `cmd/server` 装配依赖（repo / cache / lock 等）。

## 依赖方向

```text
cmd → httpapi → module/*/http → module/*
module/* → port、pkg/*
infra → 实现 port（不反向依赖 module）
```

跨模块调用优先通过对方的 `Service` 公开方法，不要直接依赖对方的 `repo` 实现。  
例如 `team` 通过 `user.Service.GetByID` 展示创建人，不直接访问 `user/repo`。

## 日志（与 module 的关系）

- 使用 `internal/pkg/logger`（`Module("<name>")` + `purpose` / `event`），不要在 module 里直接依赖 `infra` 或 `log` 标准库刷屏。
- **Service** 打关键写操作与审计点（如 `user.registered`、`team.joined`）；可预期业务错误返回 `response.BizError` 即可。
- **Job / 预热**（如 `cache_task.go`）打任务生命周期；缓存细节可用 `Debug`。
- **Handler** 一般只 `RespondError` / `RespondOK`；注销等无 Service 落点的操作可在 Handler 记 audit。
- 细则见 [pkg/logger/README.md](../pkg/logger/README.md)。
