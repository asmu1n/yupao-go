# 内部公共库（pkg）

与具体业务模块无关、可被多处复用的代码。**不要**把业务逻辑放进这里。

| 包 | 说明 |
|----|------|
| `page` | 分页请求 / 响应 |
| `response` | 统一 API 响应体、业务错误码、Gin 写出辅助 |
| `types` | 跨模块基础类型（如 Gender、TeamStatus） |
| `logger` | 薄封装 slog：`Init` 读环境变量，业务用 `Info`/`Module` + purpose/event |

业务模块代码在 `internal/module/`；基础设施在 `internal/infra/`；端口在 `internal/port/`。
