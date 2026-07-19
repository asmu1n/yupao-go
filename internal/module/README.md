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

新增业务（例如 team）时：

1. 新建 `internal/module/team/`，按上表拆分。
2. 在 `internal/httpapi` 注册路由（或 `module/team/http` 提供 `Register`）。
3. 在 `cmd/server` 装配依赖（repo / cache / lock 等）。

## 依赖方向

```text
cmd → httpapi → module/*/http → module/*
module/* → port、pkg/*
infra → 实现 port（不反向依赖 module）
```

跨模块调用优先通过对方的 `Service` 公开方法，不要直接依赖对方的 `repo` 实现。
