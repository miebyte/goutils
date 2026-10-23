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
- `ServiceName` 是公共 `share.GetServiceName()` 的默认回退值；优先级为启动阶段显式 `share.SetServiceName`、环境变量 `ServiceName`、构建注入值，配置和服务注册统一读取 `share`。
- 值应在构建期确定，不在 Domain 中直接读取全局构建信息。
- 不把 Git Token、构建凭证或其他 Secret 注入这些变量。
- 本地开发不注入时保留 `Version=dev`，服务名可在 `main` 开始、`flags.Parse` 前通过 `share.SetServiceName("order-api")` 设置，运行期间不再变更。环境变量 `ServiceName` 保留 `name:tag` 格式，详见仓库 [share/README.md](../../../share/README.md)。
