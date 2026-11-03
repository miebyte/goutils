# 统一服务发现

业务代码通过 `discover` 按服务名查找地址，默认使用 Consul；本地调试时配置一个 JSON 服务目录即可，无需启动 Consul。

## 本地服务目录

创建 `services.json`，服务名对应一组 `host:port`：

```json
{
  "orders": ["127.0.0.1:8080", "127.0.0.1:8081"],
  "service1": []
}
```

在进程启动阶段配置默认数据源，业务调用保持不变：

```go
import (
    "context"
    "errors"

    "github.com/miebyte/goutils/discover"
)

func run(ctx context.Context) error {
    if err := discover.Configure(discover.Config{LocalFile: "services.json"}); err != nil {
        return err
    }

    addresses, err := discover.Discover(ctx, "orders")
    if err != nil {
        return err
    }
    // addresses 为 ["127.0.0.1:8080", "127.0.0.1:8081"]。
    _ = addresses

    address, err := discover.Resolve(ctx, "orders")
    if err != nil {
        return err
    }
    // address 随机选择一个目标，可交给 HTTP 或 gRPC 客户端。
    _ = address

    _, err = discover.Resolve(ctx, "service1")
    if errors.Is(err, discover.ErrNoAvailableService) {
        // 目录未包含服务或服务地址数组为空。
        return err
    }
    return err
}
```

本地模式只使用目录，缺少服务或配置 `[]` 时返回 `ErrNoAvailableService`，不会回退到 Consul。目录在 `Configure` 或 `New` 时读取并校验，之后使用内存快照；修改文件后需重新调用 `Configure` 或重启进程。初始化失败会返回错误，业务应在启动时处理。

JSON 根必须是对象，每个值必须是字符串数组，根或值为 `null` 均不合法。地址必须包含非空 host 和有效数字端口（1–65535），例如 `localhost:8080`、`127.0.0.1:8080`、`[::1]:8080`，不接受 URL scheme 或路径。配置时不进行 DNS 解析或连通性检查，因此目录中的地址需由开发者维护；它们不会经过 Consul 健康检查。

返回地址按字符串排序、去重，每次返回独立切片。`Resolve` 从去重后的地址中随机选择一个。返回值不添加 URL scheme；HTTP 客户端自行添加 `http://` 或 `https://`。

## Consul 模式

未调用 `Configure`，或使用 `discover.Config{}` 时，默认通过 Consul 发现服务：

```go
addresses, err := discover.Discover(ctx, "orders")
address, err := discover.Resolve(ctx, "orders")
```

Consul 模式复用 `consulutils.DefaultClient()`。需要限定 Kubernetes namespace 或复用自定义的官方 `*api.Client` 时，在启动阶段统一设置：

```go
err := discover.Configure(discover.Config{
    Namespace: "test",
    Client:    client, // 可省略，使用公共 Client。
})
```

默认跨 Kubernetes namespace 查询，不读取 `POD_NAMESPACE`；`Namespace` 非空时按 `Service.Meta.namespace` 精确筛选。只返回 Consul Health API 中健康实例的地址，每次查询最多等待 3 秒，并遵守调用方更短的 Context 期限；不缓存查询结果、不启动后台 Watch，也不关闭共享 Client。详细行为及 ACL 权限见 [Consul 服务发现](../consulutils/README.md#按服务名发现健康地址)。

`LocalFile` 不能与非空 `Namespace` 或非 nil `Client` 同时设置。`Source` 不能与 `LocalFile`、非空 `Namespace` 或非 nil `Client` 同时设置。调用 `discover.Configure(discover.Config{})` 可将进程默认数据源恢复为 Consul。

## 独立实例

需要为不同调用方设置不同数据源时，创建独立的 `Resolver`：

```go
resolver, err := discover.New(discover.Config{LocalFile: "services.json"})
if err != nil {
    return err
}
addresses, err := resolver.Discover(ctx, "orders")
address, err := resolver.Resolve(ctx, "orders")
```

`New` 不改变包级默认数据源，后续 `Configure` 也不改变已经创建的实例。`ErrNoAvailableService` 与 `consulutils.ErrNoAvailableService` 为同一个错误值，可用 `errors.Is` 判断；文件、配置、Consul 查询和 Context 错误保留各自原因。原有 `consulutils.Discover`、`consulutils.Resolve` 仍只使用 Consul，不受 `discover.Configure` 影响。

## 数据源接口

`Resolver` 只依赖 `Discoverer` 接口，数据源负责按服务名提供地址：

```go
type Discoverer interface {
    Discover(context.Context, string) ([]string, error)
}
```

内置的 `discover.Consul{Namespace: "test", Client: client}` 和 `discover.NewLocal("services.json")` 返回的 `*discover.Local` 都实现该接口。也可通过 `Config.Source` 注入自定义实现：

```go
type CustomSource struct{}

func (CustomSource) Discover(ctx context.Context, name string) ([]string, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }
    if name != "orders" {
        return nil, discover.ErrNoAvailableService
    }
    return []string{"127.0.0.1:8080"}, nil
}

// 在组合根创建实例，或用 Configure 设置包级默认数据源。
resolver, err := discover.New(discover.Config{Source: CustomSource{}})
if err != nil {
    return err
}
address, err := resolver.Resolve(ctx, "orders")
```

实现须支持并发调用，将传入的 Context 继续传递给底层查询，并保留原始错误；返回的地址须为有效 `host:port`。所有数据源的结果都由 `Resolver` 复制、排序和去重；无错误的空结果统一转为 `ErrNoAvailableService`。`Resolve` 同样通过该接口查询并随机选择地址。

## 运行示例

在仓库根目录运行，示例只输出地址，不发送业务请求：

```sh
# 使用本地目录，无需 Consul，也无需先启动目标服务。
go run ./example/consul-discovery -services=example/consul-discovery/services.json

# service1 的空数组会返回无可用服务错误。
go run ./example/consul-discovery -services=example/consul-discovery/services.json -target=service1

# 不指定本地目录时查询 Consul。
go run ./example/consul-discovery -target=orders
go run ./example/consul-discovery -target=orders -namespace=test
```
