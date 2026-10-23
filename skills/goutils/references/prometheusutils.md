# prometheusutils 模块

`prometheusutils` 提供独立 Prometheus Registry、`/metrics` Handler 和若干内置指标。它是横切/Infrastructure 能力。

## 最简接入

通过 `WithHttpHandler` 将业务和采集路由组显式挂载到同一个业务 HTTP 端口：

```go
srv := cores.NewCores(
    cores.WithServiceAddress(":8080"),
    cores.WithHttpHandler(
        cores.HttpHandler{ApiPrefix: "/api", Handler: handler},
        cores.HttpHandler{ApiPrefix: "/metrics", Handler: prometheusutils.PrometheusHttpHandler()},
    ),
)
logging.PanicError(cores.Start(srv))
```

上例的采集地址为 `http://127.0.0.1:8080/metrics/`；`cores` 只允许一个业务 HTTP 地址，管理端口默认仍独立监听 `:6060`。也可直接使用 `prometheusutils.PrometheusHttpHandler()` 挂到已有 HTTP Server。`cores` 不会自动挂载或采集业务 HTTP 指标；需要在业务代码或中间件中调用对应的上报函数。

## 内置指标

- `CustomCounter` / `SendCustomCounter`
- `APICounter` / `SendAPICounter`
- `APIHistogram` / `SendAPIHistogram`
- `TCPConnectionGauge`
- `RemoteConnectionGauge`

## 注意事项

- `SendAPIHistogram` 的耗时参数直接传给 Histogram，内置桶为 `100`、`500`、`1000`、`5000`；按毫秒上报并保持单位一致。
- 公共 `server_name` 是 ConstLabel，内置指标在包初始化时创建。服务名应优先通过构建期 `buildinfo.ServiceName` 注入；仅在 `flags.Parse` 后运行期设置名称，可能无法改变已创建指标的 ConstLabel。
- `SendCurrentTCPConnectionGauge` 读取 Linux `/proc/1/net/snmp`，在 macOS、Windows 或无该文件的容器中会记录错误；不要把它作为跨平台必选逻辑。
- 不使用用户 ID、订单 ID 等高基数字段作为 Prometheus Label。调用 `SendAPIHistogram` 时，`url` 应使用归一化路由模板，避免动态 ID 路径产生高基数。
- `CreateRegistry()` 返回仅注册 Go/Process Collector 的新 Registry，不会自动带上包级内置指标。
- 业务指标命名、单位和 Label 在注册前确定，避免重复注册和不兼容变更。
