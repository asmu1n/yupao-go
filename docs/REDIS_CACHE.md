# Redis 缓存策略说明

本文描述 **当前仓库实际落地** 的 Redis 使用方式与缓存策略，供协作与排障对照。  
匹配业务细节另见：[`internal/module/user/CACHE.md`](../internal/module/user/CACHE.md)。

---

## 1. 总览：Redis 在本项目中的三种角色

| 角色             | 用途         | 主要代码                            | 数据量级               |
| ---------------- | ------------ | ----------------------------------- | ---------------------- |
| **业务缓存**     | 用户匹配结果 | `infra/cache` + `module/user/match` | 随活跃用户增长，有 TTL |
| **Session 存储** | 登录态       | `infra/redis/session.go`            | 随在线会话             |
| **分布式锁**     | 预热任务互斥 | `infra/lock` + `cache_task.go`      | 通常 0～1 个 key       |

```text
                    ┌─────────────────────────────────────┐
                    │              Redis 实例              │
                    │  (同一 Host/Port，见环境变量)        │
                    └─────────────────────────────────────┘
                       ▲              ▲              ▲
                       │              │              │
              ┌────────┴───┐   ┌──────┴──────┐  ┌───┴────────┐
              │ 业务 Client │   │ Session 池  │  │ 同上 Client │
              │ go-redis    │   │ redistore×10│  │ (锁复用)    │
              └────────┬───┘   └──────┬──────┘  └─────┬──────┘
                       │              │                │
              ┌────────┴───┐   ┌──────┴──────┐  ┌─────┴──────┐
              │ port.Cache │   │ Gin Session │  │ port.Locker│
              │ + L1 TinyLFU│   │ cookie 会话 │  │ SetNX/Lua  │
              └────────────┘   └─────────────┘  └────────────┘
```

要点：

- **业务缓存与锁**共用 `cmd/server` 里创建的同一个 `redis.Client`。
- **Session** 通过 `redis.NewStore` **另开连接池**（固定 size=10），不是同一 Client 对象。
- 业务层只依赖 `port.Cache` / `port.Locker`，不直接 import Redis 实现细节。

---

## 2. 连接与客户端

### 2.1 业务客户端

| 项       | 当前实现                                                               |
| -------- | ---------------------------------------------------------------------- |
| 创建     | `internal/infra/redis.NewClient`                                       |
| 配置来源 | `REDIS_HOST` / `REDIS_PORT` / `REDIS_PASSWORD`                         |
| 显式配置 | 仅 `Addr`、`Password`                                                  |
| 连接池等 | **未自定义**，使用 go-redis 默认（约 `PoolSize = 10 × GOMAXPROCS` 等） |
| 启动探测 | `Ping`，超时 5s                                                        |

### 2.2 Session 客户端

| 项        | 当前实现                                                            |
| --------- | ------------------------------------------------------------------- |
| 创建      | `redis.NewSessionStore` → `sessions/redis.NewStore(10, "tcp", ...)` |
| 连接数    | 池大小 **10**                                                       |
| Cookie 名 | `session`（见 `cmd/server`）                                        |

### 2.3 资源含义（简）

| 资源       | 谁占用                  | 当前策略                           |
| ---------- | ----------------------- | ---------------------------------- |
| TCP 连接   | 业务池 + Session 池     | 无应用层硬顶；Session 固定 10      |
| 应用内存   | L1 缓存最多 1000 条     | 有 cap                             |
| Redis 内存 | 匹配 key + session + 锁 | **靠 TTL 回收，无 key 数量硬上限** |

---

## 3. 业务缓存（匹配结果）——核心策略

### 3.1 缓存什么

| 项           | 说明                                                                |
| ------------ | ------------------------------------------------------------------- |
| **Key**      | `yupao:match:{userID}:{num}`                                        |
| **Value**    | 推荐用户列表 `[]*User`（序列化后存 Redis）                          |
| **业务 TTL** | 基准 **60 分钟**，写入时 **最多缩短约 20%（jitter）**，减轻同时过期 |
| **num 含义** | 推荐人数；接口允许 1～20；**预热只写 10 与 20**                     |

同一用户不同 `num` 是不同 key，互不影响。

### 3.2 层级结构（L1 + L2）

```text
读请求
  → L1 进程内 TinyLFU（最多 1000 条，本地 TTL 10 分钟）
      → 未命中 → L2 Redis
          → 未命中 → 执行业务计算 → 回写 L1 + L2
```

| 层  | 实现                          | 容量 / TTL               | 作用                                |
| --- | ----------------------------- | ------------------------ | ----------------------------------- |
| L1  | `cache.NewTinyLFU(1000, 10m)` | 1000 条 / 10m            | 降 Redis 读 QPS；**只保护本机内存** |
| L2  | Redis                         | 无条数 cap / ~60m±jitter | 跨实例共享；主存储                  |

实现：`internal/infra/cache`（`go-redis/cache` 的 `Once`）。

### 3.3 读写模式：Cache-Aside + Once

对外入口：`MatchUsers` → `port.TryFetch` → `Cache.Once`。

| 情况                 | 行为                                                            |
| -------------------- | --------------------------------------------------------------- |
| **Hit**              | 反序列化直接返回，不打匹配计算                                  |
| **Miss**             | 执行 `match`：加载活跃候选 → 标签编辑距离 Top-K → 写缓存 → 返回 |
| **同 key 并发 miss** | Once 保证只计算一次，其它请求等待同一结果                       |
| **cache == nil**     | 降级为直接计算，不写缓存（便于测试）                            |

**Once 语义（重要）**：

- key **已存在则不刷新** value（预热只做「补冷」，不是「强制重算」）。
- 适合防击穿；若要每日强制刷新，需先 Delete 或提供强制 Set（当前未做）。

### 3.4 候选数据与结果一致性

在线 miss 与定时预热使用 **同一套** 逻辑：

```text
loadActiveCandidates  →  活跃池（SQL 过滤 + 游标分批）
rankMatches           →  编辑距离 + Top-K + ListByIDs
```

活跃池条件（摘要）：

- `user_status = 0`、`is_delete = 0`、`tags != ""`
- `update_time` 在近 **7 天**（`matchActiveWindow`）内
- 按 ID 游标，`warmBatchSize = 200`

**约束**：禁止预热用窄集合、在线用宽集合却写同一 key，否则缓存语义错误。

### 3.5 写入来源

| 来源         | 何时写                          | 写哪些 key                             |
| ------------ | ------------------------------- | -------------------------------------- |
| **在线请求** | 缓存 miss                       | 当前请求的 `userID + num`              |
| **定时预热** | 每天 03:00（`cmd/server` cron） | 活跃用户 × `num ∈ {10,20}`，且仅冷 key |

预热控制：

| 项       | 值                                          |
| -------- | ------------------------------------------- |
| 调度     | `0 0 3 * * *`（每天 03:00:00）              |
| 任务超时 | 约 10 分钟（入口 `context.WithTimeout`）    |
| 分布式锁 | `lock:cron:warmup_match_users`，TTL 10 分钟 |
| 进程内   | mutex + cron `SkipIfStillRunning`           |
| 并发     | `warmWorkers = 4`                           |
| 覆盖 num | 仅 10、20                                   |

### 3.6 失效策略

| 触发               | 行为                                                                     |
| ------------------ | ------------------------------------------------------------------------ |
| 用户 `Update` 成功 | `invalidateMatchCache`：删除该用户 `num = 1..20` 的全部匹配 key（L1+L2） |
| TTL 到期           | 自然删除                                                                 |
| 预热               | **不**主动删旧 key；已存在则跳过                                         |

说明：

- 失效的是「**被更新用户自己的推荐结果**」。
- 该用户出现在 **别人** 的推荐列表里时，**不会**级联删除，依赖对方 key 的 TTL。
- 多实例下 L1 只清本机；其它节点 L1 最多残留约 10 分钟本地 TTL。

### 3.7 内存与写入量（策略层面的预期）

| 维度                      | 策略现状                                       |
| ------------------------- | ---------------------------------------------- |
| 单 key 存活               | ~48～60 分钟（jitter）                         |
| 预热 key 数量上界（量级） | ≈ `活跃用户数 × 2`（仅 10/20）                 |
| 在线额外 key              | 其它 `num` 的懒加载写入                        |
| Redis 内存硬上限          | **代码未设**；依赖部署侧 `maxmemory`（若配置） |
| 写入限速                  | 仅预热 4 并发；无全局 QPS 配额                 |

粗算：`占用 ≈ key 数 × 单条序列化大小`；value 为完整用户列表，比只存 ID 更占内存。

---

## 4. Session 存储

| 项             | 说明                                                         |
| -------------- | ------------------------------------------------------------ |
| 库             | `gin-contrib/sessions` + Redis store                         |
| 作用           | 登录后保存 `userID` 等会话数据                               |
| 与业务缓存关系 | **独立 key 空间**（由 redistore 管理），不是 `yupao:match:*` |
| 连接           | 独立连接池 size=10                                           |

Session **不算业务查询缓存**，但占用同一 Redis 实例的内存与连接，容量规划时要一并考虑。

---

## 5. 分布式锁（非缓存，同实例）

| 项       | 说明                                    |
| -------- | --------------------------------------- |
| Key      | `lock:cron:warmup_match_users`          |
| 获取     | `SET key token NX EX ttl`               |
| 释放     | Lua：仅 value==token 时 DEL（防误删）   |
| TTL      | 10 分钟                                 |
| 失败语义 | `port.ErrLockFailed` → 其它节点跳过预热 |

锁 key 极少、TTL 短，对内存可忽略；用于保证预热不并行放大写压力。

---

## 6. Key 命名约定

| 前缀 / 模式                    | 用途         |
| ------------------------------ | ------------ |
| `yupao:match:{userID}:{num}`   | 匹配推荐结果 |
| `lock:cron:warmup_match_users` | 预热分布式锁 |
| Session（redistore 默认前缀）  | 登录会话     |

新增业务缓存时建议：

```text
yupao:<业务>:<实体ID>:<维度...>
```

并统一：TTL、是否 L1、失效时机、是否允许预热写同一 key。

---

## 7. 策略原则（设计结论）

1. **Cache-Aside**：业务算真源在 DB + 算法；Redis 是加速层，可 miss、可删。  
2. **Once 防击穿**：热点 key 并发只算一次。  
3. **TTL + Jitter**：控制存活时间，打散过期尖峰。  
4. **L1 降读、L2 共享**：本机热数据走内存，跨实例走 Redis。  
5. **在线与预热同源**：同一候选池 + 同一排序，避免缓存语义分裂。  
6. **主动失效范围有限**：改资料只删自己的 match key；关联推荐靠 TTL。  
7. **无应用层 Redis 配额**：生产需在 Redis 侧配置 `maxmemory` 等，或后续加预热上限 / 缩小 value。

---

## 8. 流程简图

### 8.1 在线匹配读

```text
GET /api/user/match?num=N
  → Auth（Session 读 Redis）
  → MatchUsers
       → TryFetch / Once(yupao:match:{uid}:{N})
            hit  → 返回
            miss → loadActiveCandidates → rankMatches → 写入 L1+L2 → 返回
```

### 8.2 预热写

```text
Cron 03:00
  → 抢锁 lock:cron:warmup_match_users
  → loadActiveCandidates（与在线相同）
  → 4 workers × 每用户 Once(num=10,20) 补冷
```

### 8.3 更新失效

```text
Update 用户成功
  → Delete yupao:match:{uid}:1 .. :20
```

---

## 9. 关键参数速查

| 常量 / 配置         | 默认          | 作用                   |
| ------------------- | ------------- | ---------------------- |
| `matchCacheTTL`     | 60m           | 匹配 L2 TTL 基准       |
| TTL jitter          | 最多 −20%     | 防同时过期             |
| L1 size / TTL       | 1000 / 10m    | 本机缓存               |
| `matchActiveWindow` | 7d            | 活跃候选时间窗         |
| `maxMatchNum`       | 20            | 接口上限；失效删除范围 |
| `warmUpNums`        | 10, 20        | 预热覆盖的 num         |
| `warmWorkers`       | 4             | 预热写并发             |
| `warmBatchSize`     | 200           | 候选游标批次           |
| `lockTTL`           | 10m           | 预热锁                 |
| Session pool        | 10            | Session Redis 连接数   |
| 业务 PoolSize       | go-redis 默认 | 未在代码写死           |

---

## 10. 相关代码与文档

| 路径                                 | 内容                                |
| ------------------------------------ | ----------------------------------- |
| `internal/infra/redis/`              | Client、Session、环境配置           |
| `internal/infra/cache/`              | L1+L2、Once、jitter                 |
| `internal/infra/lock/`               | 分布式锁                            |
| `internal/port/cache.go` / `lock.go` | 业务端口                            |
| `internal/module/user/match.go`      | 匹配读缓存                          |
| `internal/module/user/cache_task.go` | 预热与失效                          |
| `internal/module/user/CACHE.md`      | 匹配缓存细节与架构图                |
| `cmd/server/main.go`                 | 装配 Client / Cache / Locker / Cron |

---

## 11. 变更检查清单（改缓存时）

- [ ] 新 key 是否有 TTL？  
- [ ] 是否与现有 key 冲突？前缀是否规范？  
- [ ] 写路径与读路径计算结果是否一致（尤其预热 vs 在线）？  
- [ ] 数据变更时是否要 Delete？范围是否正确？  
- [ ] Once 是否符合预期（补冷 vs 强制刷新）？  
- [ ] 预估 key 数量 × value 大小是否可接受？  
- [ ] 是否需要更新本文与 `CACHE.md`？
