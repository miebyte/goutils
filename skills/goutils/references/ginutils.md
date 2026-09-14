# ginutils 模块

`ginutils` 属于 API/Interface 层，提供 Gin Engine、嵌套路由、请求绑定、数据清洗/校验和统一响应。

## 路由装配

当前 API 名称是单数 `WithMiddleware`、`WithRouterHandler`，以及 `WithGroupHandlers`、`WithHandler`：

```go
func (a *API) SetupRouter() http.Handler {
    return ginutils.NewServerHandler(
        ginutils.WithMiddleware(a.authMiddleware()),
        ginutils.WithGroupHandlers(
            ginutils.WithPrefix("/orders"),
            ginutils.WithRouterHandler(orderRouter(a.orders)),
        ),
    )
}
```

Router 只注册相对路径：

```go
type routerFunc func(gin.IRouter)

func (fn routerFunc) Init(r gin.IRouter) { fn(r) }

func orderRouter(app OrderApplication) ginutils.Router {
    return routerFunc(func(r gin.IRouter) {
        r.GET("/:id", getOrderHandler(app))
    })
}
```

- `api.go` 负责全局中间件和一级分组。
- 资源 Router 注入最窄 Application 接口。
- Handler 不直接访问 MySQL、Redis、队列或事务。
- 中间件和子组按声明顺序挂载。

## 请求处理

`RequestHandler[Q]` 和 `RequestResponseHandler[Q,P]` 会按字段 Tag 推断 Body、Header、URI、Query 绑定策略，然后执行 mold 数据修饰和 validator 校验。

```go
func getOrderHandler(app OrderApplication) gin.HandlerFunc {
    return ginutils.RequestResponseHandler(
        func(c *gin.Context, req *GetOrderRequest) (*OrderResponse, error) {
            return app.Get(c.Request.Context(), req.ID)
        },
    )
}
```

- 使用 `json`、`uri`、`form`、`header` 和 `validate` Tag 表达协议。
- 始终把 `c.Request.Context()` 传给 Application。
- 列表请求会逐项修饰和校验。
- 数据修饰失败当前只记录日志，校验失败会直接返回错误响应；不要把关键业务校验只放在 modifier。
- `RequestResponseHandler` 返回 `nil, nil` 时不会写响应，只有刻意自行处理响应时才这样做。

## 响应与错误

统一响应为：

```go
type Ret[T any] struct {
    Code    int `json:"code"`
    Data    T   `json:"data,omitzero"`
    Message any `json:"message,omitempty"`
}
```

- 成功业务码为 `0`。
- 默认失败业务码为 `-1`。
- 错误实现 `ErrCoder` 可提供业务码。
- 错误实现 `HTTPStatusCoder`，或经 `WithHTTPStatus` 包装，可提供 HTTP 状态码。
- 没有 HTTP 状态信息时，`ReturnError` 默认 HTTP 200；不要把业务失败码和 HTTP 状态混为一谈。

项目已有错误类型时统一映射，不为每个 Handler 重复 `switch`。

## 日志与敏感信息

- `LoggingRequest(header)` 会记录请求体，并可选择记录 Header；请求体最多记录约 1 KiB。
- Header 或 Body 可能含 Token、密码时，不要启用未脱敏的请求日志。
- `LoggerMiddleware` 负责访问日志；不要再在每个 Handler 重复记录同一条请求摘要。
