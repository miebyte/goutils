# Flags v2：Consul 配置

在 `Parse` 时传入 `WithRemoteConfig()`，即可从 Consul KV 读取已声明的 `Struct` 配置：

```go
import flags "github.com/miebyte/goutils/flags/v2"

type AppConfig struct {
    Limit int `json:"limit"`
}

var appConfig = flags.Struct("app", &AppConfig{Limit: 10}, "应用配置")

func main() {
    flags.Parse(
        flags.WithRemoteConfig(flags.WithServiceName("orders")),
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

连接使用 Consul 官方 Go SDK 的默认配置，默认地址为 `127.0.0.1:8500`，可以通过 `CONSUL_HTTP_ADDR`、`CONSUL_HTTP_TOKEN` 等标准环境变量设置。TLS、认证和数据中心等选项遵循 [SDK 配置](https://pkg.go.dev/github.com/hashicorp/consul/api#Config)。

默认配置路径为 `/configs/{serviceName}/{key}`，上述示例读取 `/configs/orders/app`。SDK 去掉开头的 `/` 后，请求 `/v1/kv/configs/orders/app`，所以 Consul 中的 KV key 为 `configs/orders/app`。值直接存放 JSON，例如 `{"limit":20}`，无需额外包一层 `"app"`。key 不存在时使用声明的默认值；连接、权限或 JSON 解析错误会中止加载。普通 Flag 仍只读取命令行和默认值，不从 Consul 读取。

服务名沿用 `WithServiceName`、环境变量 `ServiceName` 或构建注入的 `buildinfo.ServiceName`，命令行 `--serviceName/-s` 可以覆盖它。Provider 延迟到首次使用时读取服务名；未设置服务名时省略这一层，得到 `/configs/{key}`。

需要指定连接和 KV key 前缀时：

```go
import consulapi "github.com/hashicorp/consul/api"

flags.Parse(
    flags.WithRemoteConfig(
        flags.WithServiceName("orders"),
        flags.WithRemoteConsulProvider(&consulapi.Config{
            Address: "127.0.0.1:8500",
        }, "/prod"),
    ),
)
```

此时 `app` 对应 `/prod/orders/app`；空前缀使用默认 `/configs`，自定义前缀只替换根前缀，后面仍会追加服务名。独立使用 `SuperFlags` 时，也可通过 `SetConfigProvider(flags.NewConsulProvider(config, prefix))` 设置来源。

单个 Struct 可使用 `WithConfigPath("shared/app")` 替换 key 后缀，默认得到 `/configs/orders/shared/app`；使用 `WithFullConfigPath("/shared/app")` 则原样读取 `/shared/app`，不追加根前缀或服务名。本地文件模式仍按 Struct 声明的 key 读取，不使用这些远程路径选项。

`WithConfigWatch()` 使用官方 `github.com/hashicorp/consul/api/watch` 包监听 KV 变更，查询、索引和失败重试由 SDK 管理，查询错误由 SDK 记录日志。订阅等待首次快照建立后返回，不推送初值；等待超过 10 秒则停止本次订阅并返回错误。更新经现有的 JSON 解析、`SetDefault()` 和 `Validate()` 后发布为新快照；删除、读取失败或无效更新保留上一份有效快照。退出时调用 `Close()` 停止 Watch Plan 并关闭监听。显式指定 `--configFile/-f` 始终优先使用本地文件。
