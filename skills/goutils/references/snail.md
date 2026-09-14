# snail 模块

`snail` 是轻量的延迟初始化注册表：包初始化时注册函数，启动阶段统一执行。

```go
func init() {
    snail.RegisterObject("feature-init", func() error {
        return initializeFeature()
    })
}
```

`flags.Parse()` 结束时会自动调用 `snail.Init()`。未使用 `flags.Parse()` 的程序若依赖已注册对象，需要在组合根显式调用一次 `snail.Init()`。

## 约束

- 注册和执行都是全局状态，按注册顺序执行；只在初始化阶段使用。
- 任一函数返回错误会触发启动失败/panic，不适合可选能力的静默降级。
- 初始化函数应幂等、快速、无无限阻塞，不启动无人管理的长期 goroutine。
- 业务模块通常不直接依赖 `snail`；优先在 `main.go` 显式组装。只有解决 Go 包初始化顺序或框架级延迟初始化时才使用。
