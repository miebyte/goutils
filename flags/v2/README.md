# Flags v2：Consul 配置

在 `Parse` 时传入 `WithRemoteConfig()`，即可从 Consul KV 读取已声明的 `Struct` 配置：

```go
import flags "github.com/miebyte/goutils/flags/v2"

type AppConfig struct {
    Limit int `json:"limit"`
}

var appConfig = flags.Struct("app", &AppConfig{Limit: 10}, "应用配置")

func main() {
    flags.Parse(flags.WithRemoteConfig(), flags.WithConfigWatch())
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

上述示例读取 KV key `app`，值直接存放 JSON，例如 `{"limit":20}`，无需额外包一层 `"app"`。key 不存在时使用声明的默认值；连接、权限或 JSON 解析错误会中止加载。普通 Flag 仍只读取命令行和默认值，不从 Consul 读取。

需要指定连接和 KV key 前缀时：

```go
import consulapi "github.com/hashicorp/consul/api"

flags.Parse(
    flags.WithRemoteConfig(
        flags.WithRemoteConsulProvider(&consulapi.Config{
            Address: "127.0.0.1:8500",
        }, "config/orders"),
    ),
)
```

此时 `app` 对应 `config/orders/app`；前缀由调用方完整指定，不会自动追加服务名。`WithConfigPath[*AppConfig]("shared/app")` 可为单个 Struct 指定完整 KV key，跳过前缀。独立使用 `SuperFlags` 时，也可通过 `SetConfigProvider(flags.NewConsulProvider(config, prefix))` 设置来源。

`WithConfigWatch()` 使用官方 `github.com/hashicorp/consul/api/watch` 包监听 KV 变更，查询、索引和失败重试由 SDK 管理，查询错误由 SDK 记录日志。订阅等待首次快照建立后返回，不推送初值；等待超过 10 秒则停止本次订阅并返回错误。更新经现有的 JSON 解析、`SetDefault()` 和 `Validate()` 后发布为新快照；删除、读取失败或无效更新保留上一份有效快照。退出时调用 `Close()` 停止 Watch Plan 并关闭监听。显式指定 `--configFile/-f` 始终优先使用本地文件。
