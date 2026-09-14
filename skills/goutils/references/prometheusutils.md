# prometheusutils 模块

`prometheusutils` 提供独立 Prometheus Registry、`/metrics` Handler 和若干内置指标。它是横切/Infrastructure 能力。

## 最简接入

优先由 `cores` 挂载：

```go
srv := cores.NewCores(
    cores.WithHttpHandler("/", handler),
    cores.WithPrometheus(),
)
```

也可直接使用 `prometheusutils.PrometheusHttpHandler()` 挂到已有 HTTP Server。

## 内置指标

- `CustomCounter` / `SendCustomCounter`
- `APICounter` / `SendAPICounter`
- `APIHistogram` / `SendAPIHistogram`
- `TCPConnectionGauge`
- `RemoteConnectionGauge`

`cores.WithPrometheus()` 会自动统计普通 HTTP 请求，并忽略 `/metrics`、pprof 和健康检查路径。

## 注意事项

- 内置 Histogram 的耗时值由 `cores` 以毫秒上报；自定义调用保持同一单位。
- 公共 `server_name` 是 ConstLabel，内置指标在包初始化时创建。服务名应优先通过构建期 `buildinfo.ServiceName` 注入；仅在 `flags.Parse` 后运行期设置名称，可能无法改变已创建指标的 ConstLabel。
- `SendCurrentTCPConnectionGauge` 读取 Linux `/proc/1/net/snmp`，在 macOS、Windows 或无该文件的容器中会记录错误；不要把它作为跨平台必选逻辑。
- 不使用用户 ID、订单 ID 等高基数字段作为 Prometheus Label。`cores` 自动 Histogram 使用原始 `r.URL.Path` 作为 `url` Label，动态 ID 路径可能产生高基数；此类服务应改用归一化路由模板指标或评估是否启用自动统计。
- `CreateRegistry()` 返回仅注册 Go/Process Collector 的新 Registry，不会自动带上包级内置指标。
- 业务指标命名、单位和 Label 在注册前确定，避免重复注册和不兼容变更。
