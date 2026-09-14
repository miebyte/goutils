# redisutils 模块

`redisutils` 提供 go-redis Client、命名连接池、值序列化、分布式锁和函数结果缓存。它属于 Infrastructure。

## 配置与连接

```go
type RedisConfig struct {
    Server   string `json:"server"`
    Db       int    `json:"db"`
    Username string `json:"username"`
    Password string `json:"password"`
    MinSize  int    `json:"minsize"`
    MaxSize  int    `json:"maxsize"`
    PoolSize int    `json:"poolsize"`
    Timeout  int    `json:"timeout"`
}
```

`SetDefault()` 会补齐本地地址和连接池大小。`NewRedisClient` 创建时会 Ping；连接失败应让启动阶段失败，而不是带着 nil Client 运行。当前实现使用固定 5 秒读写超时，且没有把 `RedisConfig.Username` 和 `Timeout` 传入 `redis.Options`；需要 ACL 用户名或自定义超时时先核对/修正当前版本，不能只填写字段后假设生效。

```go
pool, err := redisutils.NewRedisPool(configs, redisutils.WithLazyLoad(true))
logging.PanicError(err)
defer pool.Close()

client, err := pool.GetRedis("cache")
```

- 默认连接名是 `default`。
- 新代码使用 `NewRedisPool`；`DialGoRedisPool` 已废弃。
- Lazy Load 只在避免启动时连接非必需 Redis 有明确价值时启用；必选依赖仍应启动即验证。

## 值操作

`SetValue/GetValue`、列表 Value 方法会为基础类型、`time.Time`、`time.Duration` 和 JSON 对象做转换。读取目标必须是指针。

- 对协议稳定性敏感的数据，明确序列化格式和版本，不依赖隐式 JSON 永久兼容。
- Key 由 Infrastructure 统一构造，不在 Domain 或多个 Application 包中散落字符串拼接。
- 所有命令传递调用方 Context。

## 分布式锁

```go
if err := client.TryLockWithTimeout(ctx, key, 30*time.Second, 2*time.Second); err != nil {
    return err
}
defer func() { _ = client.Unlock(context.WithoutCancel(ctx), key) }()
```

- `expiration` 必须覆盖临界区最长合理耗时，同时避免锁永久存在。
- `Unlock` 会校验当前进程保存的锁值；不要用另一个 Client 实例释放锁。
- 锁不自动解决业务幂等，仍需唯一约束、幂等键或状态机。
- 明确处理 `ErrLockAcquireFailed`、`ErrLockTimeout` 等错误，不进行无上限忙等。

## 函数缓存

```go
cached := redisutils.CacheCall(loadOrder).
    WithClient(client).
    SetPrefix("order").
    SetTTL(5 * time.Minute).
    OnceFunc()
```

- 函数签名必须是 `func(context.Context, Req) (Ret, error)`。
- `Req` 和 `Ret` 必须可序列化。
- `Func` 普通缓存；`OnceFunc` 使用 singleflight 合并同进程同 key 并发回源。
- Key 构建或缓存读写失败会回退原函数；业务错误不会写缓存。
- TTL 必须根据业务陈旧容忍度设置；需要防雪崩时使用 TTL jitter。
- 缓存属于性能优化，不应成为维持领域正确性的唯一存储。
