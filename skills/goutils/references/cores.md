# cores 模块

`cores` 负责进程生命周期、业务 HTTP Server、gRPC Server、Worker、优雅退出，以及内部端口的健康检查、pprof 和 Swagger。它属于组合根/bootstrap，不应进入 Domain。

## 选择启动方式

- `cores.Run(srv)` 启动所有已配置的业务 HTTP、gRPC 和 Worker，不开启内部 HTTP 端口。
- `cores.Start(srv, internalPortOrAddress)` 在上述服务之外额外开启内部 HTTP 端口，用于 `/health_check`、已启用的 pprof 和 Swagger。
- `Start` 的内部地址接受端口整数或监听地址字符串，例如 `6060`、`"127.0.0.1:6060"`，不再用于业务 HTTP。
- 业务 HTTP 由 `WithHttpHandler` 指定地址，gRPC 由 `WithGrpcServer` 指定地址，均与内部 HTTP 端口独立。

```go
httpConfig := &cores.HttpServerConfig{
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
}
httpConfig.SetDefault()

srv := cores.NewCores(
    cores.WithHttpServerConfig(httpConfig),
    cores.WithHttpHandler(":8080", "/api", api.SetupRouter()),
    cores.WithWorker(worker.Run),
    cores.WithPprof(),
)
logging.PanicError(cores.Start(srv, "127.0.0.1:6060"))
```

## HTTP 挂载

`WithHttpHandler(addr string, pattern string, handler http.Handler)` 会：

- 在 `Run` 或 `Start` 时监听 `addr`，业务请求只在该地址上提供。
- 多次使用相同 `addr` 时共享 Listener 和 ServeMux，使用不同 `addr` 时创建独立业务 HTTP Server。
- 自动补齐前导和结尾 `/`。
- 通过 `http.StripPrefix` 去掉挂载前缀后再交给 Handler。

因此最终路径由 cores 挂载前缀和 Handler 内部路由共同组成。不要在 Gin Router 中重复写被 Strip 的前缀。

`/health_check`、`WithPprof()` 的 `/debug/pprof/` 和 `WithSwagger()` 的文档路由仅位于 `Start` 的内部端口。`Run` 不会提供这些内部端点。业务端口不会自动注册健康检查。

Prometheus 需要显式挂载 Handler，例如：

```go
cores.WithHttpHandler("127.0.0.1:9090", "/metrics", prometheusutils.PrometheusHttpHandler())
```

采集路径为 `/metrics/`。`cores` 不会自动采集业务 HTTP 指标，具体上报方式见 [prometheusutils](prometheusutils.md)。

`WithHttpCORS()` 已弃用；CORS 策略应在业务 Web 框架或中间件中配置。

旧调用 `WithHttpHandler("/api", handler)` 需改为 `WithHttpHandler(":8080", "/api", handler)`；原先传给 `Start` 的业务地址移到 `WithHttpHandler`，`Start` 改传独立内部地址，或直接使用 `Run`。

## gRPC 挂载

```go
grpcServer := grpc.NewServer()
pb.RegisterUserServiceServer(grpcServer, userService)

srv := cores.NewCores(
    cores.WithGrpcServer(":9000", grpcServer),
)
logging.PanicError(cores.Run(srv))
```

- `WithGrpcServer(addr string, server *grpc.Server)` 接收已创建且非 nil 的原生 gRPC Server；在启动前完成所有 protobuf 服务注册。同一个 Server 可以注册多个服务。
- 一个 `CoresService` 配置一个 gRPC Server。`cores` 在 `Run`/`Start` 时创建并持有 TCP Listener，负责调用 `Serve`、关闭 Listener 和停止 Server；调用方不再自行调用 `Serve` 或管理该 Server 的停机。
- TLS、Unary/Stream interceptor、keepalive 等通过 `grpc.NewServer(...)` 的原生选项配置，`cores` 不另加一层配置。
- 同时提供 HTTP 时，增加 `WithHttpHandler(":8080", "/api", handler)`，继续使用 `cores.Run(srv)`；gRPC 继续监听 `:9000`。需要内部端点时再改为 `cores.Start(srv, "127.0.0.1:6060")`。
- 任一端口绑定失败都会返回错误，并回收本次已创建的 Listener；HTTP、gRPC 或 Worker 发生运行错误时会触发统一退出，将错误返回给 `Run`/`Start`。有 HTTP 或 gRPC 服务时，短 Worker 正常结束不会使进程退出。
- 收到退出信号或运行错误后，HTTP 和 gRPC 并行关闭，完成后再取消并等待 Worker。gRPC 先调用 `GracefulStop` 等待在途 RPC 完成，超过默认 5 秒后调用 `Stop` 强制关闭并返回包含 `context.DeadlineExceeded` 的错误；使用 `WithDefaultMaxWait` 调整该期限。长时间运行的流式 RPC 也受此期限约束。
- 停机期限约束 `cores` 的等待时间；RPC handler 应响应自身 Context 的取消，忽略取消的 handler 可能在 `cores` 返回后继续运行。
- `WithPprof`、`WithSwagger` 和 HTTP `/health_check` 依赖 `Start` 的内部端口。gRPC health service、reflection 按业务需要自行注册。

仓库的 `example/cores-grpc` 使用标准 gRPC health service，不需要生成 protobuf 文件，并注册 reflection 方便使用 `grpcurl` 验证。

```sh
# 纯 gRPC，默认监听 :9000
go run ./example/cores-grpc

# 或同时启动 HTTP :8080 和 gRPC :9000
go run ./example/cores-grpc --http-address=:8080

# 同时启动业务 HTTP、gRPC，以及内部健康检查和 pprof
go run ./example/cores-grpc --http-address=:8080 --internal-address=127.0.0.1:6060

# 在另一个终端验证 gRPC（需要已安装 grpcurl）
grpcurl -plaintext -d '{"service":""}' 127.0.0.1:9000 grpc.health.v1.Health/Check

# 验证业务 HTTP
curl http://127.0.0.1:8080/hello/

# 指定 --internal-address 后验证内部端点
curl http://127.0.0.1:6060/health_check
curl http://127.0.0.1:6060/debug/pprof/
```

gRPC health 检查返回 `SERVING`；业务 HTTP 示例返回 `hello`，内部 HTTP 健康检查返回 `ok`。按 Ctrl+C 可触发优雅退出。示例默认使用明文 gRPC；部署时按实际需要配置 TLS 和认证拦截器。

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
- 不重复实现 OS Signal 监听；`cores` 已负责取消 Context 和关闭 HTTP/gRPC Server。
