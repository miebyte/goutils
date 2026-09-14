# cores 模块

`cores` 负责进程生命周期、HTTP Server、Worker、优雅退出、健康检查、pprof 和 Prometheus 挂载。它属于组合根/bootstrap，不应进入 Domain。

## 选择启动方式

- HTTP 或 HTTP + Worker：`cores.Start(srv, portOrAddress)`。
- 纯 Worker：`cores.Run(srv)`。
- `Start` 接受端口整数或监听地址字符串，例如 `8080`、`":8080"`。

```go
httpConfig := &cores.HttpServerConfig{
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
}
httpConfig.SetDefault()

srv := cores.NewCores(
    cores.WithHttpServerConfig(httpConfig),
    cores.WithHttpHandler("/api", api.SetupRouter()),
    cores.WithWorker(worker.Run),
    cores.WithPrometheus(),
)
logging.PanicError(cores.Start(srv, port))
```

## HTTP 挂载

`WithHttpHandler(pattern, handler)` 会：

- 自动补齐前导和结尾 `/`。
- 通过 `http.StripPrefix` 去掉挂载前缀后再交给 Handler。
- 在 `cores.Start` 时额外注册 `/health_check`。

因此最终路径由 cores 挂载前缀和 Handler 内部路由共同组成。不要在 Gin Router 中重复写被 Strip 的前缀。

`WithPprof()` 和 `WithPrometheus()` 依赖 HTTP Listener，只在 `cores.Start` 下有效；`cores.Run` 不会提供 `/debug/pprof/` 或 `/metrics`。

`WithHttpCORS()` 使用全开放 CORS。只有产品策略明确允许时才启用；更复杂的 CORS 规则应在 Web 框架中实现。

## Worker

```go
func Run(ctx context.Context) error {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            // 调用 Application Service
        }
    }
}
```

- 使用 `WithWorker(fn)` 自动取函数名，或 `WithNameWorker(name, fn)` 指定稳定名称。
- Worker 返回非 `context.Canceled` 错误会终止整个服务组。
- 默认优雅等待时间为 5 秒；可用单个 Worker 的 `maxWait` 参数或 `WithDefaultMaxWait` 调整。
- `WithWaitAllDone()` 会无限等待所有 Worker 自行退出；只在能保证 Worker 响应取消时使用。

## 约束

- 配置、连接池和 Application Service 在创建 `CoresService` 前完成初始化；传给 `WithHttpServerConfig` 的指针必须非 nil，并先调用 `SetDefault()` 补齐零值字段。
- Worker 只做触发与消费适配，业务规则放 Application/Domain。
- 不在 Worker 中吞掉不可恢复错误并无限循环。
- 不重复实现 OS Signal 监听；`cores` 已负责取消 Context 和关闭 HTTP Server。
