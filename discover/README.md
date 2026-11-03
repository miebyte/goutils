# 统一服务发现

业务通过 `discover.Discover(ctx, name)` 获取排序、去重后的 `host:port`，或通过 `discover.Resolve(ctx, name)` 随机选择一个地址。导入 `discover` 后，`flags/v2.Parse()` 自动初始化数据源：默认使用 Consul，本地调试可通过启动参数切换 JSON 服务目录。

## 启动与调用

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/miebyte/goutils/discover"
    "github.com/miebyte/goutils/flags/v2"
)

func main() {
    flags.Parse()
    defer flags.Close()

    address, err := discover.Resolve(context.Background(), "orders")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(address)
}
```

保存为服务的 `main.go` 后运行：

```sh
# 默认查询 Consul。
go run .

# 使用当前工作目录的 ./services.json，无需 Consul。
go run . --use-local-discovery=true
```

`flags.Parse` 先完成参数与配置读取，再运行 `snail` 初始化。`discover` 注册的数据源初始化回调会先于依赖它的业务包注册的 `snail` 回调执行，因此业务回调中即可进行服务发现。已通过 `SetDefaultResolver` 显式安装的数据源始终优先，自动初始化不会覆盖它，包括指定本地参数时。

服务注册与发现独立：`cores.WithConsulRegister()` 只负责注册当前服务，不设置发现数据源。

## 本地目录

在程序运行目录创建 `services.json`：

```json
{
  "orders": ["127.0.0.1:8080", "127.0.0.1:8081"],
  "service1": []
}
```

本地模式只使用该目录，缺少服务或配置 `[]` 时返回 `discover.ErrNoAvailableService`，不会回退到 Consul。文件在初始化时读取为内存快照，修改后重启进程，或再次调用 `discover.UseLocalSource()` 加载。自动初始化及 `UseLocalSource` 读取、校验失败时会停止启动。

JSON 根必须是对象，每个值必须是字符串数组，根或值为 `null` 均不合法。地址须包含非空 host 和 1–65535 的数字端口，例如 `localhost:8080`、`127.0.0.1:8080`、`[::1]:8080`，不能带 URL scheme 或路径。本地目录不执行 DNS 解析、连通性检查或 Consul 健康检查，地址由开发者维护。

需要自定义文件路径并自行处理初始化错误时：

```go
source, err := discover.NewLocal("./dev/services.json")
if err != nil {
    return err
}
discover.SetDefaultResolver(source)
```

## Consul 与自定义数据源

默认 Consul 数据源复用 `consulutils.DefaultClient()`。它查询所有 namespace 的同名健康服务，不读取 `POD_NAMESPACE`；只使用实例注册的 `Service.Address` 和端口，不取 Node 地址。单次请求最多 3 秒，并遵守调用方更短的 Context 期限；不缓存查询结果、不启动后台 Watch，也不关闭共享 Client。Health API 需要 `node:read`、`service:read` 权限。

需要自定义官方 `*api.Client` 时，在 `flags.Parse` 前安装它；需要注册或远程配置时，将同一个 Client 传给相应入口：

```go
discover.SetDefaultResolver(discover.NewConsulDiscovery(client))
flags.Parse(flags.WithRemoteConfig(flags.WithRemoteConsulProvider(client)))

srv := cores.NewCores(
    cores.WithServiceAddress(":8080"),
    cores.WithHttpHandler(cores.HttpHandler{ApiPrefix: "/", Handler: handler}),
    cores.WithConsulRegister(consulutils.RegistrationConfig{Client: client}),
)
```

数据源实现以下接口，可通过 `discover.SetDefaultResolver(source)` 注入：

```go
type Discoverer interface {
    Discover(context.Context, string) ([]string, error)
}
```

实现应支持并发调用，将 Context 传递给底层查询，保留原始错误，并返回有效 `host:port`。包级 `Discover` 统一复制、排序、去重结果，将无错误的空结果转为 `ErrNoAvailableService`。可通过 `errors.Is(err, discover.ErrNoAvailableService)` 判断无可用地址。

未使用 `flags.Parse` 的程序，应先手动调用 `SetDefaultResolver` 或 `UseLocalSource`，再调用服务发现。返回地址不添加 URL scheme，也不会自动更新已有 HTTP/gRPC 连接；HTTP 客户端自行添加 `http://` 或 `https://`。
