# 公共进程信息

`share` 提供进程统一的服务名，以及现有的 Tag、Debug 和主机名信息。配置路径、服务注册和生命周期日志都从这里读取服务名。

```go
import (
    "github.com/miebyte/goutils/flags/v2"
    "github.com/miebyte/goutils/share"
)

func main() {
    // 必须在解析配置和组装服务之前设置，运行期间不再修改。
    share.SetServiceName("orders")
    flags.Parse(flags.WithRemoteConfig())
    defer flags.Close()

    name := share.GetServiceName()
    _ = name
}
```

服务名优先级从高到低为：启动阶段显式调用 `share.SetServiceName`、环境变量 `ServiceName`、构建注入的 `buildinfo.ServiceName`。未显式设置时保留环境和构建信息的回退行为；`flags` 的两个版本都只读取公共服务名，不负责声明或覆盖它。

环境变量保留 `name:tag` 的历史格式，例如 `ServiceName=orders:v1.2.3` 会设置服务名 `orders` 和 Tag `v1.2.3`，按第一个冒号分隔。未提供环境 Tag 时，Tag 默认来自 `buildinfo.Version`。显式 `SetServiceName("orders")` 只设置服务名，不修改 Tag；需要调整 Tag 时继续使用已有 `share.SetTag`。

服务名是启动信息，应在 `main` 开始、`flags.Parse` 之前固定。配置 Provider、Consul 注册和日志初始化后不再修改，避免同一进程出现不同服务身份。
