# buildinfo 模块

`buildinfo` 暴露构建期可注入变量：

```go
var (
    Version     = "dev"
    ServiceName = ""
)
```

推荐在构建时使用 `-ldflags -X`：

```bash
go build -ldflags "\
  -X github.com/miebyte/goutils/buildinfo.Version=v1.2.3 \
  -X github.com/miebyte/goutils/buildinfo.ServiceName=order-api"
```

- `Version` 同时会作为默认 service tag 使用。
- `ServiceName` 会被 `flags`、`cores` 和 `prometheusutils` 读取。
- 值应在构建期确定，不在 Domain 中直接读取全局构建信息。
- 不把 Git Token、构建凭证或其他 Secret 注入这些变量。
- 本地开发不注入时保留 `Version=dev`，服务名可由 `--serviceName` 或环境配置补充；注意 Prometheus ConstLabel 的包初始化时机，详见 [prometheusutils.md](prometheusutils.md)。
