# logging 模块

`logging` 提供全局 PrettyLogger、上下文字段、结构化字段、Formatter 和 Hook。它是横切能力，Domain 中只在确有价值且不会形成基础设施耦合时使用。

## 优先使用 Context 日志

```go
ctx = logging.With(ctx, "OrderID", orderID)
logging.Infoc(ctx, "order created")
logging.Errorw(ctx, "payment failed", logging.Field{Key: "provider", Value: provider})
```

- `Infoc/Debugc/Warnc/Errorc`：Context + 格式化参数。
- `Infos/Debugs/Warns/Errors`：Context + key/value 参数。
- `Infow/Debugw/Warnw/Errorw`：Context + `Field`。
- 无 Context 的 `Infof` 等仅用于启动阶段或确实没有调用链的代码。

继续传递原 Context，不要用 `context.Background()` 丢失已有字段。

## Logger 配置

- `SetLogger` 替换全局 Logger。
- `NewPrettyLogger` 创建模块专用 Logger。
- `WithModule` 增加模块名，`WithEnableSource` 控制源码位置。
- `SetFormatter`、`SetOutput`、`Enable` 修改全局输出行为。
- `AddGlobalHook`/`AddHook` 用于确有复用价值的日志处理，不要在业务包中反复注册。

全局配置应在 bootstrap 一次完成，避免运行期并发修改 Logger。导入 `logging` 会把进程级 `time.Local` 设置为固定 UTC+8 的 `CST`；依赖系统时区或 DST 的服务必须显式评估这一全局副作用。

## 约束

- 不记录密码、Token、Cookie、Secret、私钥和完整个人敏感信息。
- 输出结构体前确认其字段安全；不要直接 `Jsonify` 整份配置。
- 业务错误日志通常在最能补充上下文的一层记录一次，避免 API、Application、Repository 层层重复。
- `PanicError` 只用于启动阶段不可恢复错误；请求处理和 Worker 单次任务不要用 panic 代替错误返回。
