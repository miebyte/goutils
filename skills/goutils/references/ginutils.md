# ginutils 模块

`ginutils` 属于 API/Interface 层，提供 Gin Engine、嵌套路由、请求绑定、数据清洗/校验和统一响应。

## 路由装配

`api.go` 只做装配：全局中间件、一级分组、资源 Router 的挂载；资源 Router 只注册相对路径与 Handler。当前 API 名称是单数 `WithMiddleware`、`WithRouterHandler`，以及 `WithGroupHandlers`、`WithHandler`。

### 最小装配

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

### 典型装配：免鉴权入口与鉴权分组并列

```go
func (api *API) SetupRouter() http.Handler {
    engine := ginutils.NewServerHandler(
        ginutils.WithMiddleware(
            middleware.RecoveryMiddleware(),      // 框架默认 recovery 会返回业务成功码
            middleware.NoCacheMiddleware(),
            middleware.OriginMiddleware(api.config.Origin),
            observeOperations(),
        ),
        ginutils.WithHandler(http.MethodGet, "/v1/status", statusHandler(api.version)),
        // 无需会话的入口单独成组
        ginutils.WithGroupHandlers(
            ginutils.WithPrefix("/v1/auth"),
            ginutils.WithRouterHandler(router.AuthRouter(api.authApp, api.sessionOptions())),
        ),
        // 需要会话与成员校验的部分
        ginutils.WithGroupHandlers(
            ginutils.WithPrefix("/v1"),
            ginutils.WithMiddleware(middleware.TokenVerifyMiddleware(api.authApp)),
            ginutils.WithRouterHandler(router.MeRouter(), router.GroupRouter(api.groupApp)),
            ginutils.WithGroupHandlers(
                ginutils.WithPrefix("/groups/:group"),
                ginutils.WithRouterHandler(router.RoundRouter(api.roundApp)),
            ),
        ),
    )
    engine.HandleMethodNotAllowed = true
    engine.NoRoute(noRouteHandler)
    engine.NoMethod(noMethodHandler)
    return engine
}
```

- 分组中间件对子分组同样生效，所以「免鉴权入口」必须挂在与鉴权组并列的位置。
- 每个资源 Router 用 `type xxxRouterFn func(gin.IRouter)` + `Init` 适配 `ginutils.Router`。
- Handler 内从统一的位置取当前身份（context 或 session 中间件写入的账号），不要在 Handler 里重新查库鉴权。
- 处理器按需选择 `RequestHandler`（只绑定请求）或直接 `func(c *gin.Context)`（需要自己写响应头、Cookie 或文件流时）。
- 生成型入口（`cmd/swagger`）与 `go:generate` 注释一起提交，`make swagger` 可重复生成；生产用 `is_prod` 之类的配置关闭文档路由。
- 设置 `HandleMethodNotAllowed` 时，同时注册 `NoRoute`/`NoMethod`，让未知路径与方法返回明确错误，不被 SPA fallback 掩盖。

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
- 校验失败与绑定失败默认返回 HTTP 200 + 失败业务码；要区分「客户端请求非法」，需要在项目里统一包装相应错误。

## 错误与响应

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
- 错误实现 `ErrCoder`（`Code() int`）可提供业务码。
- 错误实现 `HTTPStatusCoder`（`HTTPStatus() int`），或经 `WithHTTPStatus` 包装，可提供 HTTP 状态码。
- 没有 HTTP 状态信息时，`ReturnError` 默认 HTTP 200；不要把业务失败码和 HTTP 状态混为一谈。

推荐的错误契约（业务码与 HTTP 状态各管一件事）：

```go
type Error struct {
    ErrCode int
    status  int
    Message string
}

func (e Error) Error() string   { return e.Message }
func (e Error) Code() int       { return e.ErrCode }   // ginutils.ErrCoder
func (e Error) HTTPStatus() int { return e.status }    // ginutils.HTTPStatusCoder
```

业务码用稳定分段（如 `100400`/`100401`/`100404`/`100409`/`100500`），预定义错误集中在 `internal/errcode`，领域与用例只返回这些值；API 层用一个统一出口映射响应：

```go
// api/common
func HandleRouterError(c *gin.Context, err error, logMsg string, fallback errcode.Error) bool {
    if err == nil {
        return false
    }
    if ec, ok := errcode.AsErrcode(err); ok {
        ginutils.ReturnError(c, ec)
        return true
    }
    if logMsg != "" {
        logging.Errorc(c.Request.Context(), "%s: %v", logMsg, err)
    }
    ginutils.ReturnError(c, fallback)
    return true
}
```

- Handler 只写 `if common.HandleRouterError(c, err, "… failed", errcode.ErrXxx) { return }`，不为每个接口重复 `switch`。
- 面向浏览器的接口建议保留显式 HTTP 状态（401/403/404/405/409/429/502/503），前端以业务码判断成败、以状态处理传输层失败；纯内网或 SDK 场景可以统一 200 + 业务码，但要在项目文档里写清取舍。
- 兜底 recover 也走同一出口：`gin.New()` 之外自行注册 recovery 时，panic 必须转成明确的失败响应。

## 日志与敏感信息

- `LoggingRequest(header)` 会记录请求体，并可选择记录 Header；请求体最多记录约 1 KiB。
- Header 或 Body 可能含 Token、密码时，不要启用未脱敏的请求日志。
- `LoggerMiddleware` 负责访问日志；不要再在每个 Handler 重复记录同一条请求摘要。
- 会话、身份等请求级字段建议由中间件写入 `context`（如 `logging.With(ctx, "UserID", id)`），便于后续日志统一带上下文。
