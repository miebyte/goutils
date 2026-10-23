# Flags v2：Consul 配置

在 `Parse` 时传入 `WithRemoteConfig()`，即可从 Consul KV 读取已声明的 `Struct` 配置：

```go
import (
    flags "github.com/miebyte/goutils/flags/v2"
    "github.com/miebyte/goutils/share"
)

type AppConfig struct {
    Limit int `json:"limit"`
}

var appConfig = flags.Struct("app", &AppConfig{Limit: 10}, "应用配置")

func main() {
    share.SetServiceName("orders")
    flags.Parse(
        flags.WithRemoteConfig(),
        flags.WithConfigWatch(),
    )
    defer flags.Close()

    config, err := appConfig()
    if err != nil {
        panic(err)
    }
    current := config.Load() // 每次使用时读取当前只读快照
    _ = current.Limit
}
```

默认通过公共包 `consulutils.DefaultClient()` 获取同一个官方 SDK Client，配置读取、配置 Watch 和服务注册共享其 HTTP 连接池。Client 首次使用时读取 SDK 默认配置，默认地址为 `127.0.0.1:8500`，可以通过 `CONSUL_HTTP_ADDR`、`CONSUL_HTTP_TOKEN` 等标准环境变量设置。TLS、认证和数据中心等选项遵循 [SDK 配置](https://pkg.go.dev/github.com/hashicorp/consul/api#Config)。环境变量应在进程启动前配置，运行中修改不会替换已经创建的公共 Client。

默认配置路径为 `/configs/{serviceName}/{key}`，上述示例读取 `/configs/orders/app`。SDK 去掉开头的 `/` 后，请求 `/v1/kv/configs/orders/app`，所以 Consul 中的 KV key 为 `configs/orders/app`。值直接存放 JSON，例如 `{"limit":20}`，无需额外包一层 `"app"`。key 不存在时使用声明的默认值；连接、权限或 JSON 解析错误会中止加载。普通 Flag 仍只读取命令行和默认值，不从 Consul 读取。

服务名统一通过公共 `share` 包读取，应在 `main` 开始、`flags.Parse` 前显式调用 `share.SetServiceName("orders")`。未显式设置时，依次回退到环境变量 `ServiceName` 和构建注入的 `buildinfo.ServiceName`；环境变量原有 `name:tag` 格式保留。Provider 延迟到首次使用时读取服务名，未设置服务名时省略这一层，得到 `/configs/{key}`。服务名只在启动阶段设置，不在运行期间变更，详见 [share](../../share/README.md)。

需要与服务注册共用一个自定义连接时，在组合根创建一次 Client：

```go
share.SetServiceName("orders")
clientConfig := consulapi.DefaultConfig()
clientConfig.Address = "127.0.0.1:8500"
client, err := consulapi.NewClient(clientConfig)
if err != nil {
    panic(err)
}

flags.Parse(
    flags.WithRemoteConfig(
        flags.WithRemoteConsulProvider(client),
    ),
)
defer flags.Close()

srv := cores.NewCores(
    cores.WithServiceAddress(":8080"),
    cores.WithHttpHandler(cores.HttpHandler{ApiPrefix: "/", Handler: handler}),
    cores.WithConsulRegister(consulutils.RegistrationConfig{
        Client: client,
    }),
)
if err := cores.Start(srv); err != nil {
    panic(err)
}
```

其中 `consulapi` 是 `github.com/hashicorp/consul/api` 的导入别名；注册配置中的 `Client` 与配置 Provider 持有同一个对象，服务名统一取 `share.GetServiceName()`，注册端口取业务实际监听端口。默认连接无需传入注册配置，直接使用 `cores.WithConsulRegister()` 即可。省略 `WithRemoteConsulProvider()` 的参数时仍使用公共 Client；不传该选项也使用公共 Client。独立使用 `SuperFlags` 时，也可通过 `SetConfigProvider(flags.NewConsulProvider(client))` 设置来源。默认根路径固定为 `/configs`，无需为共享连接重复创建 Client。

单个 Struct 可使用 `WithConfigPath("shared/app")` 替换 key 后缀，默认得到 `/configs/orders/shared/app`；使用 `WithFullConfigPath("/shared/app")` 则原样读取 `/shared/app`，不追加根前缀或服务名。本地文件模式仍按 Struct 声明的 key 读取，不使用这些远程路径选项。

`WithConfigWatch()` 使用官方 `github.com/hashicorp/consul/api/watch` 包监听 KV 变更，查询、索引和失败重试由 SDK 管理，查询错误由 SDK 记录日志。订阅等待首次快照建立后返回，不推送初值；等待超过 10 秒则停止本次订阅并返回错误。更新经现有的 JSON 解析、`SetDefault()` 和 `Validate()` 后发布为新快照；删除、读取失败或无效更新保留上一份有效快照。退出时调用 `Close()` 只停止此 Provider 的 Watch，不关闭共享 Client 或注销服务。显式指定 `--configFile/-f` 始终优先使用本地文件。

Consul 服务注册是 `cores.WithConsulRegister` 的独立可选能力；注册请求失败会后台重试，不影响业务服务运行。远程配置属于启动依赖，连接、权限或解析错误仍会使初始化失败。完整服务示例见 [example/cores-consul](../../example/cores-consul/main.go)，注册地址和 Pod 健康检查说明见 [consulutils](../../consulutils/README.md)。
