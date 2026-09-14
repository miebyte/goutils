# breaker 模块

`breaker` 实现 Google SRE 风格熔断器，用于 Infrastructure 中保护外部网络或不稳定依赖。不要把熔断状态写进 Domain。

## 常用方式

```go
err := breaker.DoCtx(ctx, "payment-provider", func() error {
    return client.Charge(ctx, req)
})
```

按名称调用会通过全局注册表复用同一 Breaker。名称必须稳定并能区分依赖或调用场景，不要包含用户 ID、请求 ID 等高基数字段。

## 可接受错误与降级

- `Acceptable` 决定某个返回错误是否计为成功，例如业务“未找到”不代表依赖故障。
- `Fallback` 只在熔断器拒绝请求时执行，不是所有调用错误的通用兜底。
- 优先使用带 Context 的 `DoCtx`、`DoWithAcceptableCtx`、`DoWithFallbackCtx`。

```go
err := breaker.DoWithFallbackAcceptableCtx(
    ctx,
    "catalog",
    callRemote,
    loadStaleCache,
    func(err error) bool { return err == nil || errors.Is(err, ErrNotFound) },
)
```

降级结果必须符合产品语义，不能把真实失败静默伪装成成功。

## 手动模式

调用 `Allow/AllowCtx` 获得 `Promise` 后，每条允许请求必须准确调用一次 `Accept()` 或 `Reject(reason)`；遗漏会破坏统计。没有特殊控制需求时直接使用 `Do*` 更安全。

## 约束

- `ErrServiceUnavailable` 表示熔断器开启，可在 Adapter 边界映射成应用错误。
- `NoBreakerFor` 会全局禁用指定名称，只用于明确的测试或运维策略，不在业务请求中动态切换。
- 熔断器不是超时器或重试器；外部 Client 仍需设置超时，重试仍需上限和幂等保障。
