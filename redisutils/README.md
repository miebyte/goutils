# RedisUtils 工具包

RedisUtils 是一个简化 Redis 操作的 Go 工具包，基于 `github.com/redis/go-redis/v9` 构建，提供了更便捷的 Redis 客户端使用方式，包括自动类型转换、分布式锁、连接池管理等功能。

## 特性

- **简化的客户端初始化**：通过配置对象快速创建 Redis 客户端
- **自动类型转换**：支持多种数据类型的自动序列化和反序列化
- **分布式锁实现**：提供可靠的分布式锁机制
- **连接池管理**：方便管理多个 Redis 实例
- **OpenTelemetry 集成**：内置链路追踪支持
- **完整的类型安全操作**：支持字符串、整数、浮点数、布尔值、时间和结构体等类型

## 基本用法

### 初始化客户端

```go
// 创建配置
conf := &redisutils.RedisConfig{
    Host: "localhost",
    Port: 6379,
    Db: 0,
}
// 或使用默认配置
conf := &redisutils.RedisConfig{}
conf.SetDefault()

// 创建客户端
client, err := conf.DialGORedisClient()
if err != nil {
    panic(err)
}
defer client.Close()
```

### 多实例管理

```go
configMap := redisutils.RedisConfigMap{
    "default": &redisutils.RedisConfig{
        Host: "localhost",
        Port: 6379,
    },
    "cache": &redisutils.RedisConfig{
        Host: "cache.example.com",
        Port: 6380,
    },
}

// 初始化连接池
pool, err := configMap.DialGoRedisPool()
if err != nil {
    panic(err)
}
defer pool.Close()

// 获取指定实例
defaultClient, err := pool.GetRedis() // 默认实例
cacheClient, err := pool.GetRedis("cache") // 指定实例
```

### 值操作

```go
ctx := context.Background()

// 存储不同类型的值
err := client.SetValue(ctx, "string-key", "hello world", time.Hour)
err = client.SetValue(ctx, "int-key", 123, time.Hour)
err = client.SetValue(ctx, "struct-key", Person{Name: "张三", Age: 30}, time.Hour)

// 读取值
var strVal string
err = client.GetValue(ctx, "string-key", &strVal)

var intVal int
err = client.GetValue(ctx, "int-key", &intVal)

var person Person
err = client.GetValue(ctx, "struct-key", &person)

// 删除值
err = client.DeleteValue(ctx, "string-key")
```

### 列表操作

```go
// 添加到列表
err := client.LPushValue(ctx, "my-list", "string value", 123, true, MyStruct{})

// 从列表中获取值
var strVal string
err = client.LPopValue(ctx, "my-list", &strVal)

// 获取列表范围
var items []string
err = client.RangeValue(ctx, "my-list", 0, -1, &items)
```

### 分布式锁

```go
key := "my-lock"

// 尝试获取锁
err := client.TryLock(ctx, key, time.Second*30)
if err != nil {
    // 处理锁获取失败
    return
}

// 带超时的锁获取
err = client.TryLockWithTimeout(ctx, key, time.Second*30, time.Second*5)

// 业务逻辑...

// 释放锁
err = client.Unlock(ctx, key)
```

## 示例

完整示例请参考 `example/main.go`：

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/miebyte/goutils/logging"
    "github.com/miebyte/goutils/redisutils"
)

type Person struct {
    Name string
    Age  int
}

func main() {
    conf := &redisutils.RedisConfig{}
    conf.SetDefault()

    client, err := conf.DialGORedisClient()
    if err != nil {
        panic(err)
    }

    // 向列表中推入多种类型的值
    err = client.LPushValue(
        context.Background(),
        "test-push",
        "string value",
        123,
        true,
        123.456,
        time.Now(),
        Person{Name: "张三", Age: 30},
    )
    
    // 从列表中获取这些值
    var stringRet string
    client.RPopValue(context.Background(), "test-push", &stringRet)
    fmt.Printf("stringRet: %v\n", stringRet)
    
    // ... 继续获取其他类型
}
```

## 高级特性

### 自定义类型支持

RedisUtils 对以下类型提供了原生支持：

- 字符串 (`string`)
- 字节数组 (`[]byte`)
- 整数类型 (`int`, `int8`, `int16`, `int32`, `int64`)
- 无符号整数 (`uint`, `uint8`, `uint16`, `uint32`, `uint64`)
- 浮点数 (`float32`, `float64`)
- 布尔值 (`bool`)
- 时间 (`time.Time`)
- 时间间隔 (`time.Duration`)
- IP地址 (`net.IP`)

对于其他自定义类型，会使用 JSON 序列化和反序列化。如果类型实现了 `encoding.BinaryUnmarshaler` 接口，将使用该接口进行处理。

## 注意事项

- 所有 Get 相关的操作都要求传入指针类型
- 分布式锁在程序异常退出时可能无法自动释放，建议使用 defer 语句确保锁的释放
- 对于高并发场景，请合理设置连接池大小
