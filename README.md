# Go Utils

本项目是一个 Go 语言工具库脚手架，包含了在开发过程中常用的各种功能模块。

## 统一服务名

在 `main` 开始时通过公共 `share` 包设置服务名，再解析配置和启动服务：

```go
share.SetServiceName("orders")
flags.Parse(flags.WithRemoteConfig())
```

配置路径、Consul 注册和生命周期日志统一读取 `share.GetServiceName()`。服务名优先级为显式设置、环境变量 `ServiceName`、`buildinfo.ServiceName`，只在启动阶段设置，运行期间不再修改。环境变量原有 `name:tag` 格式仍保留，详见 [share](share/README.md)。

## 服务与脚本启动

`cores.Start(srv)` 用于长期运行的 HTTP、gRPC 和 Worker 服务，默认在 `:6060` 提供管理 HTTP，包括 `/health_check` 存活检查和 `/ready` 就绪检查；pprof 和 Swagger 按需启用。需要修改管理地址时，在构造阶段传入 `cores.WithInternalHTTPAddress(":6061")`，启动函数不再接收端口。

`cores.Run(srv)` 用于一次性脚本，只运行 Worker，全部完成后退出，不启动管理 HTTP，也不执行 Consul 注册。业务 HTTP、gRPC 必须使用服务模式。迁移已有代码时，将 `Start(srv, addr)` 改为构造选项加 `Start(srv)`，将原来通过 `Run` 启动的长期服务改为 `Start`。

- [HTTP 与 Worker 示例](example/cores/main.go)
- [gRPC 示例](example/cores-grpc/main.go)
- [一次性脚本示例](example/cores-script/main.go)
- [完整生命周期说明](skills/goutils/references/cores.md)

## 可选 Consul 注册

配置读取、Watch 和服务注册默认复用公共 `consulutils.DefaultClient()`。注册需显式添加 `cores.WithConsulRegister`，失败后台重试，不终止业务进程；远程配置读取失败仍按启动依赖处理。

业务地址统一通过 `cores.WithServiceAddress(":8080")` 设置，HTTP 使用 `cores.WithHttpHandler(cores.HttpHandler{ApiPrefix: "/api", Handler: handler}, ...)` 挂载路由组，gRPC 使用 `cores.WithGrpcServer(grpcServer)` 挂载服务。配置 HTTP 或 gRPC 时必须提供业务地址；二者共用一个业务 Listener，由 cmux 按连接分流。共享模式使用明文 HTTP/gRPC，TLS 在入口代理终止，管理端口仍独立。

`cores.WithConsulRegister()` 可直接无参启用，服务名取进程当前服务名，注册端口取唯一业务 Listener 的实际端口；只需在覆盖 Client 等可选参数时传入一份注册配置。Kubernetes Service 的 `port` 需要与该业务端口一致。

集群通过部署注入的 `POD_NAMESPACE`、`POD_NAME`、`POD_IP` 识别，三者必须同时提供：注册 Kubernetes Service 域名，Consul `Name` 仍取 `share` 服务名，实例 ID 固定为 `namespace-podName-businessPort`，不额外添加逻辑服务名前缀。健康检查指向当前 Pod IP 的管理端口。本地默认注册 `127.0.0.1` 和业务端口，实例 ID 规则保持不变；同机运行多个服务时需分别设置管理端口。

- [注册、共享连接与 Kubernetes 部署配置](consulutils/README.md)
- [配置读取与 Watch](flags/v2/README.md)
- [Consul 服务示例](example/cores-consul/main.go)

## 推荐项目目录架构
```
.
├── api
│   ├── api.go
│   ├── handler
│   ├── logic
│   └── middlewares
├── cmd
│   ├── cron
│   ├── generate
│   ├── server
│   └── swagger
├── config
├── dal
├── main.go
├── model
│   ├── constant
│   ├── dto
│   └── types
├── pkg
├── README.md
├── service
├── superconf.json
└── worker

```
