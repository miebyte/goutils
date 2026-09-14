# debounce 模块

`debounce.LessExecutor` 用于在给定时间窗口内最多执行一次函数，其余调用直接丢弃。

```go
executor := debounce.NewLessExecutor(5 * time.Second)
executed := executor.DoOrDiscard(func() {
    refresh()
})
```

- 返回 `true` 表示本次执行，`false` 表示被丢弃。
- 时间值的单次读写是原子的，但“读取后判断再写入”不是一个原子操作；多个 goroutine 同时越过阈值时可能都执行。严格要求并发场景最多执行一次时，在外层加锁或使用 CAS 方案。
- 这是“限频并丢弃”，不会延迟并合并到窗口末尾，也不返回任务错误。
- 不用于必须执行的业务命令、资金操作或可靠消息；适合重复日志、刷新提示、非关键通知等可丢弃动作。
