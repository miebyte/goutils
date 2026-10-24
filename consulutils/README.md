# Consul 公共 Client、服务注册与发现

`consulutils` 为配置读取、服务注册和发现提供公共的官方 `*api.Client`。`flags/v2` 的 Consul Provider、配置 Watch、`cores.WithConsulRegister` 和服务发现默认使用同一个 Client 和 HTTP 连接池，不要求业务代码重复创建连接。

## 连接配置

```go
client, err := consulutils.DefaultClient()
if err != nil {
    return err
}
```

Client 首次使用时读取官方 SDK 的默认配置和 `CONSUL_HTTP_ADDR`、`CONSUL_HTTP_TOKEN` 等环境变量，默认地址为 `127.0.0.1:8500`。环境配置应在进程启动前完成，运行期间修改环境变量不会替换已有公共 Client。TLS、数据中心和认证等配置遵循 [Consul Go SDK](https://pkg.go.dev/github.com/hashicorp/consul/api#Config)。

需要自定义连接时，在组合根创建一个官方 Client，同时传给 `flags.WithRemoteConsulProvider(client)`、`RegistrationConfig.Client` 和 `DiscoveryConfig.Client`，详见 [flags/v2 示例](../flags/v2/README.md)。停止配置 Watch 不会注销服务，注销服务也不会关闭配置 Watch；服务发现不关闭共享 Client。

**服务注册必须使用稳定的同一个 Consul Agent 地址。** `Agent().ServiceRegister()`、查询已注册实例和注销请求操作具体 Agent，普通负载均衡地址可能让这些请求落到不同 Agent；仅复用同一个 Client 不能解决这种问题。配置 KV 读取和服务注册共用该稳定地址，ACL Token 需要相应 KV 读取权限以及注册服务的 `service:read`、`service:write` 权限。参见 [Consul Agent Service API](https://developer.hashicorp.com/consul/api-docs/agent/service)。

## 在 cores 中启用注册

```go
share.SetServiceName("orders")
srv := cores.NewCores(
    cores.WithServiceAddress(":8080"),
    cores.WithHttpHandler(cores.HttpHandler{ApiPrefix: "/", Handler: handler}),
    cores.WithConsulRegister(),
)
logging.PanicError(cores.Start(srv))
```

注册默认关闭；添加 `WithConsulRegister()` 后，仅在 `Start` 服务模式执行。服务名统一读取 `share.GetServiceName()`，业务端口由框架实际绑定的 Listener 取得，无需重复指定，也不能通过注册配置覆盖。在 `main` 开始、`flags.Parse` 前调用 `share.SetServiceName("orders")`；未显式设置时，依次回退到环境变量 `ServiceName` 和 `buildinfo.ServiceName`。环境变量原有 `name:tag` 格式保留；服务名只在启动阶段设置，运行期间不变，详见 [share](../share/README.md)。

完成监听初始化后后台注册，请求失败只记录日志并重试，业务服务继续运行。失败默认从 5 秒开始指数退避，最长 1 分钟，并加入抖动；成功后默认每 5 秒确认记录仍存在，Agent 重启或记录被移除时重新注册。重试始终复用当前进程的实例 ID。

注册的健康检查初始为 `critical`，由 Consul 成功访问该实例的 `/ready` 后转为健康。退出时先停止注册任务，用独立超时注销当前实例；异常退出依赖 Consul 健康检查将记录标为不健康并最终移除。配置错误（例如 Pod 环境变量不完整、业务端口无效）只记录日志并停用本次注册，不使服务退出；这类错误需要修正部署配置。Consul 不可用时的后台重试不会改变远程配置初始化失败即停止启动的策略。

`Run` 是脚本模式，不开启管理 HTTP，也不执行服务注册。健康检查默认使用服务模式的管理地址，默认 `:6060`，可通过实例选项 `cores.WithInternalHTTPAddress(":6061")` 覆盖。`/health_check` 检查进程存活，`/ready` 表示框架监听已初始化且尚未开始退出；不自动检查业务依赖。pprof 仍需显式启用 `cores.WithPprof()`。

## 注册配置

常用情况直接调用 `WithConsulRegister()`；自定义连接、地址或检查参数时，最多传入一个 `consulutils.RegistrationConfig`：

```go
cores.WithConsulRegister(consulutils.RegistrationConfig{
    Client: client,
    Tags:   []string{"dev"},
})
```

| 字段 | 含义与默认值 |
| --- | --- |
| `Client` | 可选；省略使用公共 Client |
| `ServiceAddress` | 显式覆盖注册地址的 host，不含 scheme 或端口 |
| `Tags` / `Meta` | 注册标签及元数据；集群自动记录 `namespace`、`pod_name` |
| `RequestTimeout` | 单次 Consul 请求超时，默认 3 秒 |
| `RetryInterval` | 重试起始间隔及成功后的确认间隔，默认 5 秒 |
| `CheckInterval` | Consul HTTP 检查间隔，默认 10 秒 |
| `CheckTimeout` | Consul HTTP 检查超时，默认 2 秒 |
| `DeregisterCriticalServiceAfter` | 持续不健康后的自动注销时间，默认 10 分钟 |

注册使用业务实际监听端口，健康检查使用管理端口，两者独立。集群中 Kubernetes Service 的 `port` 必须与业务实际监听端口一致；`RegistrationConfig` 不再另设端口。健康检查 host 在集群固定使用 `POD_IP`，本地固定使用 `127.0.0.1`，不能单独覆盖；`ServiceAddress` 只覆盖注册地址，不改变健康检查地址。Consul 必须能访问自动生成的检查地址。仅运行消费者的服务仍可使用 `Start` 提供健康接口；只有实际需要被其他服务发现并调用时才开启注册。

## Kubernetes 环境

通过 Downward API 给应用容器注入三个环境变量：

```yaml
env:
  - name: POD_NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace
  - name: POD_NAME
    valueFrom:
      fieldRef:
        fieldPath: metadata.name
  - name: POD_IP
    valueFrom:
      fieldRef:
        fieldPath: status.podIP
  - name: CONSUL_HTTP_ADDR
    value: "http://127.0.0.1:8500" # 示例：与应用同 Pod 的 Agent；按实际部署替换
readinessProbe:
  httpGet:
    path: /ready
    port: 6060
livenessProbe:
  httpGet:
    path: /health_check
    port: 6060
```

以上是容器配置片段，`CONSUL_HTTP_ADDR` 必须按实际部署提供稳定 Agent 地址，示例 localhost 仅适用于同 Pod Agent。集群中 `POD_NAMESPACE`、`POD_NAME`、`POD_IP` 必须同时提供；三者全部缺省时按本地环境处理，全部存在且有效时才按集群处理，部分存在时停用注册并记录错误，避免将 `127.0.0.1` 错误注册到集群。默认管理服务监听 `:6060`，kubelet 和 Consul Agent 必须能访问 Pod IP 的该端口。环境变量来源见 [Kubernetes Downward API](https://kubernetes.io/docs/concepts/workloads/pods/downward-api/)。

例如 namespace 为 `test`、Pod 名为 `orders-65fd77bb5f-abcde`、Service 名为 `orders`、业务端口为 `8080` 时：

| 内容 | 结果 |
| --- | --- |
| Consul 服务名 | `orders` |
| 注册地址 | `orders.test.svc.cluster.local:8080` |
| 实例 ID | `test-orders-65fd77bb5f-abcde-8080` |
| HTTP 健康检查 | `http://<当前 Pod IP>:6060/ready` |

集群默认注册地址固定按 `{进程服务名}.{namespace}.svc.cluster.local` 生成，Kubernetes Service 名沿用进程服务名，集群域固定为 `cluster.local`；需要显式指定其他注册地址时，可使用 `ServiceAddress`。多个 Pod 注册相同 Service 域名，并使用各自 Pod IP 检查自身状态；调用流量仍由 Kubernetes Service 分配到后端 Pod。业务端口由实际 Listener 推导，不单独覆盖。

集群 Consul `Name` 仍使用 `share.GetServiceName()`，实例 ID 固定为 `namespace-podName-businessPort`，不额外添加逻辑服务名前缀。不同 Pod 名区分不同实例；同一 namespace 下同名 Pod 以相同业务端口重建时会复用该 ID。本地实例 ID 规则保持不变。

同一个 Consul 中不同 namespace 的同名服务仍属于同一个逻辑服务名。服务发现默认跨 namespace 查询，需要隔离时显式传入 `DiscoveryConfig.Namespace`；唯一实例 ID 不自动隔离环境。通过 Consul 得到域名后，调用方仍需要能解析并访问该集群域名。

## 按服务名发现健康地址

```go
addresses, err := consulutils.Discover(ctx, "orders")
if err != nil {
    return err
}
// addresses 是已去重、排序的 host:port，例如 orders.test.svc.cluster.local:8080。

address, err := consulutils.Resolve(ctx, "orders")
if err != nil {
    return err
}
// address 是从健康地址中随机选择的一个，可交给 HTTP 或 gRPC 客户端。
```

`Discover` 查询 Consul Health API，只返回健康服务的 `Service.Address` 和 `Service.Port`，不回退到 Node 地址；空地址或非法端口会被忽略。结果按 `host:port` 去重并排序，支持 `[IPv6]:port`；集群中多个 Pod 注册同一个 Service 域名时只返回一份地址。`Resolve` 重新查询并从这些去重后的地址中随机选择一个。函数返回注册地址，不补域名、不添加 URL scheme，也不提前做 DNS 解析；HTTP 调用方自行添加 `http://` 或 `https://`。

返回结果是当前查询快照，已创建的 HTTP/gRPC 客户端连接不会自动更新；调用方应复用客户端或连接，避免每次 RPC 都重新 Dial，需要刷新目标时自行安排查询和连接更新。

**默认不限 Kubernetes namespace，也不读取 `POD_NAMESPACE`。** `DiscoveryConfig.Namespace` 非空时，才对注册的 `Service.Meta.namespace` 做精确匹配；这里指应用注册元数据，不是 Consul Enterprise Namespace。最多传入一份可选配置，自定义 Client 和 namespace 可在同一份配置中提供：

```go
addresses, err := consulutils.Discover(ctx, "orders", consulutils.DiscoveryConfig{
    Namespace: "test",
    Client:    client, // 可省略，默认使用公共 Client。
})
```

每次查询最多等待 3 秒，调用方 Context 的更短期限优先生效。不缓存结果、不启动后台 Watch，也不无限重试。无可用地址时返回 `ErrNoAvailableService`，可通过 `errors.Is(err, consulutils.ErrNoAvailableService)` 判断；空结果的响应若带有 ACL 过滤标记，错误提示也会指出 ACL 过滤。查询、权限和 Context 错误保留原始原因，不统一改成“没有服务”。

Health API 查询需要 ACL 的 `node:read` 和 `service:read` 权限，结果也受 ACL 可见范围限制。参见 [Consul Health Service API](https://developer.hashicorp.com/consul/api-docs/health#list-service-instances-for-service)。

可运行的最小示例仅查询并输出 HTTP/gRPC 目标，不发送业务请求：

```sh
# 默认查询当前 Consul 中 orders 的健康地址，不限 Kubernetes namespace
go run ./example/consul-discovery -target=orders

# 只查注册元数据 namespace=test 的地址
go run ./example/consul-discovery -target=orders -namespace=test
```

## 本地开发与示例

本地启动独立的 Consul dev Agent，再启动所需服务；每个服务默认注册 `127.0.0.1` 和业务实际监听端口，每个进程有独立实例 ID。健康检查固定使用 `127.0.0.1` 和管理端口，因此本地 Consul 必须能通过该地址访问应用；容器或其他主机中的 Consul 需确保其网络环境满足这一条件。

```sh
# 先在另一个终端启动本机 Consul（需要已安装 consul）
consul agent -dev

# 可选写入远程配置；未写入时使用 Struct 默认值
consul kv put configs/orders/app '{"message":"hello from consul"}'

# 业务 HTTP :8080，管理 HTTP :6060；开启注册和远程配置监听
go run ./example/cores-consul --consul-register

# 第二个本地实例覆盖业务端口和管理端口，注册端口自动使用实际业务端口
go run ./example/cores-consul --consul-register --port=8081 --internal-address=:6061

# 使用本地配置文件，可单独验证 Consul 不可用时注册失败仍提供业务接口
go run ./example/cores-consul --consul-register -f example/cores-consul/config.json

# 检查当前实例的管理接口
curl http://127.0.0.1:6060/ready

# 一次性脚本不占用管理端口，不执行服务注册
go run ./example/cores-script
```

可在 Consul 不可用时运行本地配置示例，确认业务 HTTP 仍可调用；恢复同一个 Agent 后，等待后台注册成功。滚动发布时检查不同 Pod 的实例 ID 独立，退出的 Pod 只删除自己的记录。Consul HTTP 检查支持与参数见 [官方健康检查文档](https://developer.hashicorp.com/consul/docs/register/health-check/vm)。
