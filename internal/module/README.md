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
