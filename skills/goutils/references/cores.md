# cores 模块

`cores` 负责进程生命周期、业务 HTTP Server、gRPC Server、Worker、优雅退出，以及管理端口的健康检查、pprof 和 Swagger。它属于组合根/bootstrap，不应进入 Domain。

## 选择启动方式

- `cores.Start(srv)` 是服务模式：启动业务 HTTP、gRPC 和 Worker，并且始终监听默认管理地址 `:6060`。长期消费者、定时 Worker 等没有业务端口的服务也使用 `Start`。
- `cores.Run(srv)` 是脚本模式：只运行 Worker，所有任务完成后退出，不监听管理端口，也不执行 Consul 注册。配置了业务 HTTP 或 gRPC 时返回错误，避免脚本模式意外常驻。
- 管理地址由构造选项 `cores.WithInternalHTTPAddress(":6061")` 覆盖，`Start` 不再接收端口参数。本地同时运行多个服务时，每个实例使用不同管理端口。
- 业务地址统一通过 `WithServiceAddress(addr)` 指定，`WithHttpHandler` 只挂载 HTTP 路由组，`WithGrpcServer` 只挂载 gRPC Server。整个服务只绑定一个业务 Listener，同时启用两种协议时由 cmux 按连接分流。管理端口始终独立。未配置业务地址却挂载 HTTP/gRPC、或必需地址绑定失败时，启动返回错误；Consul 注册失败不采用这个策略。

```go
httpConfig := &cores.HttpServerConfig{
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
}
httpConfig.SetDefault()

srv := cores.NewCores(
    cores.WithHttpServerConfig(httpConfig),
    cores.WithServiceAddress(":8080"),
    cores.WithHttpHandler(
        cores.HttpHandler{ApiPrefix: "/api", Handler: api.SetupRouter()},
        cores.HttpHandler{ApiPrefix: "/other", Handler: otherHandler},
    ),
    cores.WithWorker(worker.Run),
    cores.WithPprof(),
)
logging.PanicError(cores.Start(srv))
```

## HTTP 挂载

`WithHttpHandler(handlers ...cores.HttpHandler)` 一次挂载多个路由组，每组通过 `ApiPrefix` 和 `Handler` 指定前缀及处理器：

- 在 `Start` 时监听 `WithServiceAddress` 配置的唯一业务地址，HTTP 和 gRPC 共用该 Listener。
- 多次调用 `WithHttpHandler` 只追加路由组，共享同一个 ServeMux。`WithServiceAddress` 重复配置相同地址可接受，配置不同地址时启动返回错误，不隐式覆盖。
- 自动补齐前导和结尾 `/`。
- 通过 `http.StripPrefix` 去掉挂载前缀后再交给 Handler。

`ApiPrefix` 是字面路径前缀，空字符串等同根路径，不接受 `{}` 或空白字符等 ServeMux 通配符、方法语法；业务 Router 内部仍可按自身规则定义动态路由。

因此最终路径由 cores 挂载前缀和 Handler 内部路由共同组成。不要在 Gin Router 中重复写被 Strip 的前缀。

例如上面的 `/api/users` 进入 `api.SetupRouter()` 后路径为 `/users`，`/other/status` 进入 `otherHandler` 后路径为 `/status`；两个 Handler 在同一端口按前缀分发，不会互相接管对方的路由。可运行的多组挂载示例见 `example/cores`。

```sh
go run ./example/cores -f example/cores/config.json
# 在另一个终端分别验证 Gin 路由组和普通 HTTP Handler
curl http://127.0.0.1:8080/api/error
curl http://127.0.0.1:8080/other/status
```

`Start` 的管理端口默认提供 `/health_check` 存活检查和 `/ready` 就绪检查。完成监听初始化后 `/ready` 返回 200；开始退出时返回 503，表示不再接收新流量。它反映框架生命周期，不自动检查数据库、缓存等业务依赖。

`WithPprof()` 的 `/debug/pprof/` 和 `WithSwagger()` 的文档路由也只位于管理端口，均需显式开启。`Run` 不提供这些端点，业务端口不会自动注册健康检查。

Prometheus 需要显式挂载 Handler，例如：

```go
cores.NewCores(
    cores.WithServiceAddress(":8080"),
    cores.WithHttpHandler(
        cores.HttpHandler{ApiPrefix: "/api", Handler: apiHandler},
        cores.HttpHandler{ApiPrefix: "/metrics", Handler: prometheusutils.PrometheusHttpHandler()},
    ),
)
```

采集路径为 `/metrics/`。`cores` 不会自动采集业务 HTTP 指标，具体上报方式见 [prometheusutils](prometheusutils.md)。

`WithHttpCORS()` 已弃用；CORS 策略应在业务 Web 框架或中间件中配置。

迁移旧调用时，将 `Start(srv, internalAddress)` 改为 `Start(srv)`，在 `NewCores` 中使用 `WithInternalHTTPAddress(internalAddress)` 覆盖默认值。原来通过 `Run` 启动的 HTTP、gRPC 和长期 Worker 一并改为 `Start`。业务地址统一移入 `WithServiceAddress(":8080")`；旧 HTTP 调用改为 `WithHttpHandler(cores.HttpHandler{ApiPrefix: "/api", Handler: handler})`，旧 gRPC 调用改为 `WithGrpcServer(grpcServer)`，移除各自地址参数。管理端口仍独立于业务端口。

## gRPC 挂载

```go
grpcServer := grpc.NewServer()
pb.RegisterUserServiceServer(grpcServer, userService)

srv := cores.NewCores(
    cores.WithServiceAddress(":9000"),
    cores.WithGrpcServer(grpcServer),
)
logging.PanicError(cores.Start(srv))
```

- `WithGrpcServer(server *grpc.Server)` 接收已创建且非 nil 的原生 gRPC Server；在启动前完成所有 protobuf 服务注册。同一个 Server 可以注册多个服务，监听地址由 `WithServiceAddress` 统一配置。
- 一个 `CoresService` 配置一个 gRPC Server。`cores` 在 `Start` 时创建并持有唯一业务 TCP Listener，负责调用 `Serve`、关闭 Listener 和停止 Server；调用方不再自行调用 `Serve` 或管理该 Server 的停机。
- Unary/Stream interceptor、keepalive 等通过 `grpc.NewServer(...)` 的原生选项配置，`cores` 不另加一层配置。只提供 gRPC 时，仍可使用原生 gRPC TLS 配置。
- 同时提供 HTTP 时，增加 `WithHttpHandler(cores.HttpHandler{ApiPrefix: "/api", Handler: handler})`。cmux 在统一业务端口上按连接识别协议；当前共享模式使用明文 HTTP/gRPC，TLS 应在入口代理终止。管理 HTTP 默认独立监听 `:6060`。
- 任一端口绑定失败都会返回错误，并回收本次已创建的 Listener；HTTP、gRPC 或 Worker 发生运行错误时会触发统一退出，将错误返回给 `Start`。服务模式下，短 Worker 正常结束不会使进程退出。
- 收到退出信号或运行错误后，HTTP 和 gRPC 并行关闭，完成后再取消并等待 Worker。gRPC 先调用 `GracefulStop` 等待在途 RPC 完成，超过默认 5 秒后调用 `Stop` 强制关闭并返回包含 `context.DeadlineExceeded` 的错误；使用 `WithDefaultMaxWait` 调整该期限。长时间运行的流式 RPC 也受此期限约束。
- 停机期限约束 `cores` 的等待时间；RPC handler 应响应自身 Context 的取消，忽略取消的 handler 可能在 `cores` 返回后继续运行。
- 混合协议模式不额外跟踪连接；仍处于 cmux 协议识别阶段的连接由 `HttpServerConfig.ReadHeaderTimeout` 限制，默认 5 秒，退出时可能额外等待该识别超时。
- gRPC health service、reflection 按业务需要自行注册；它们与管理 HTTP 的健康接口独立。

仓库的 `example/cores-grpc` 使用标准 gRPC health service，不需要生成 protobuf 文件，并注册 reflection 方便使用 `grpcurl` 验证。

```sh
# 业务 gRPC 默认监听 :9000，管理 HTTP 默认监听 :6060
go run ./example/cores-grpc

# 或在 :9000 同时提供 HTTP 和 gRPC
go run ./example/cores-grpc --http

# 覆盖管理地址，并显式启用 pprof
go run ./example/cores-grpc --http --internal-address=:6061 --pprof

# 在另一个终端验证 gRPC（需要已安装 grpcurl）
grpcurl -plaintext -d '{"service":""}' 127.0.0.1:9000 grpc.health.v1.Health/Check

# 验证相同业务端口上的 HTTP（需要 --http）
curl http://127.0.0.1:9000/hello/

# 默认管理端点（覆盖为 :6061 后需使用对应端口）
curl http://127.0.0.1:6060/health_check
curl http://127.0.0.1:6060/ready
# 需要 --pprof
curl http://127.0.0.1:6060/debug/pprof/
```

gRPC health 检查返回 `SERVING`；业务 HTTP 示例返回 `hello`，管理 HTTP 健康检查返回 `ok`。通过 `--address=:9001` 同时更改两种协议的业务端口。按 Ctrl+C 可触发优雅退出。示例使用明文 gRPC；共享模式在入口代理配置 TLS，认证拦截器仍使用原生 gRPC 选项。

## 可选 Consul 注册

```go
srv := cores.NewCores(
    cores.WithServiceAddress(":8080"),
    cores.WithHttpHandler(cores.HttpHandler{ApiPrefix: "/", Handler: handler}),
    cores.WithConsulRegister(),
)
logging.PanicError(cores.Start(srv))
```

只在显式添加 `WithConsulRegister()` 时注册。服务名统一沿用进程服务名（flags 或构建注入），注册端口由业务实际监听端口推导，不再另外传入服务名和端口。需要自定义 Client、注册地址、标签或检查间隔等时，最多传入一个 `consulutils.RegistrationConfig`。监听初始化完成后后台注册；Consul 请求失败只记录日志并重试，不终止业务服务。退出时先停止重试、将就绪状态设为未就绪，再注销当前实例。脚本模式忽略该注册选项。

默认复用 `consulutils.DefaultClient()`，与 `flags/v2` 的配置读取和 Watch 共用同一个官方 SDK Client 和 HTTP 连接池。自定义 Client 时，同时传给 `flags.WithRemoteConsulProvider(client)` 和 `RegistrationConfig.Client`。关闭配置 Watch 不关闭注册器；注册失败策略不改变远程配置初始化失败时的行为。

集群通过 `POD_NAMESPACE`、`POD_UID`、`POD_IP` 三个环境变量识别，必须全部提供。默认注册地址固定按 `{进程服务名}.{namespace}.svc.cluster.local` 生成，Kubernetes Service 名沿用进程服务名，集群域固定为 `cluster.local`；显式 `ServiceAddress` 仍可覆盖注册地址。端口使用业务实际监听端口，因此 Kubernetes Service 的 `port` 必须与其一致；实例 ID 包含 Pod UID。健康检查固定指向 `http://{POD_IP}:{管理端口}/ready`，本地则固定使用 `127.0.0.1`，不支持单独覆盖检查 host；Consul 必须能访问自动生成的检查地址。

本地默认注册 `127.0.0.1` 和业务实际监听端口，使用进程实例 ID 区分多次启动；Consul 必须能访问该实例的管理 HTTP。字段说明和 Downward API 配置见仓库 [consulutils/README.md](../../../consulutils/README.md)，可运行的组合示例见 `example/cores-consul`。

## 一次性脚本

```go
srv := cores.NewCores(
    cores.WithNameWorker("rebuild", func(ctx context.Context) error {
        return app.Rebuild(ctx)
    }),
)
logging.PanicError(cores.Run(srv))
```

`Run` 在全部 Worker 完成后返回错误或成功结果，不占用默认管理端口。可运行 `go run ./example/cores-script` 查看最小示例。长期运行的队列消费者和定时任务仍属于服务模式，使用 `Start`。

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
