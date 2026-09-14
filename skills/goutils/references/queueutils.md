# queueutils 模块

`queueutils` 提供统一 `Queue[T]` 接口、内存 FIFO、优先队列和 Redis 队列。它通常作为 Infrastructure 适配器使用。

## 选择实现

- `MemoryQueue[T]`：进程内 FIFO，非持久化。
- `PriorityQueue[T PriorityItem]`：进程内、并发安全的优先队列。
- `RedisQueue[T Item]`：Redis List 持久化队列，元素用 JSON 编解码。

```go
type Queue[T any] interface {
    Enqueue(T) error
    Dequeue() (T, error)
    IsEmpty() (bool, error)
    Size() (int, error)
}
```

Application 若只需要队列语义，应依赖自己的窄接口，不直接依赖具体实现。

## 注意事项

- `MemoryQueue` 没有锁，不适合无外部同步的多 goroutine 并发访问。
- `PriorityQueue` 元素实现 `Priority() int`；默认高优先级先出，可用 `LowPriorityFirst` 改变。
- `RedisQueue` 元素实现 `Key() string`，但当前入队逻辑只序列化整个元素；不要假设 `Key()` 自动用于去重。
- `NewRedisQueue` 会自行创建 Redis Client 且 Queue 没有 `Close`；服务中优先用 `NewRedisQueueWithClient` 注入由组合根统一管理的 Client。
- `RedisQueue.Dequeue` 使用 5 秒 `BLPop`，空队列返回 `QueueEmptyError`。
- 当前 RedisQueue 内部使用 `context.Background()`，无法接收调用方取消；对严格取消、超时或消费语义有要求时，应在 Infrastructure 层封装或选择更合适的队列实现。
- 这些实现没有确认、重投、死信、可见性超时等消息系统语义；不能替代可靠消息队列。
