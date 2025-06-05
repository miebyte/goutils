# go-utils/logging

golang日志工具包，基于Go标准库的`log/slog`进行封装，提供结构化、上下文感知的日志记录功能。

## 特性

- 支持多种日志格式：JSON、文本和美化输出
- 支持上下文感知的日志记录
- 集成了日志级别控制（DEBUG、INFO、WARN、ERROR）
- 支持日志文件轮转（基于lumberjack）
- 支持将日志发送到Kafka
- 支持Sentry错误捕获
- 提供实用工具函数（计时、JSON序列化等）

## 快速开始

### 基本使用

```go
package main

import (
    "github.com/miebyte/goutils/logging"
)

func main() {
    // 使用默认日志记录器
    logging.Infof("这是一条信息日志")
    logging.Debugf("这是一条调试日志")
    logging.Warnf("这是一条警告日志")
    logging.Errorf("这是一条错误日志")
    
    // 格式化日志
    logging.Infof("用户 %s 登录成功", "张三")
}
```

### 使用上下文

```go
package main

import (
    "context"
    "github.com/miebyte/goutils/logging"
)

func main() {
    // 创建带有分组和键值对的上下文
    ctx := context.Background()
    ctx = logging.With(ctx, "用户服务")
    ctx = logging.With(ctx, "用户ID", "12345")
    
    // 使用上下文记录日志
    logging.Infoc(ctx, "用户登录成功")
    // 输出: [用户服务] 用户登录成功. "用户ID"="12345"
}
```

### 自定义日志记录器

```go
package main

import (
    "os"
    "github.com/miebyte/goutils/logging"
    "github.com/miebyte/goutils/logging/slog"
    "github.com/miebyte/goutils/logging/level"
)

func main() {
    // 创建JSON格式的日志记录器
    logger := slog.NewSlogJsonLogger(os.Stdout)
    logging.SetLogger(logger)
    
    // 设置日志级别
    logging.Enable(level.LevelDebug)
    
    // 记录日志
    logging.Debugf("现在可以看到调试日志了")
}
```

### 日志文件轮转

```go
package main

import (
    "github.com/miebyte/goutils/logging"
)

func main() {
    // 配置日志文件轮转
    logConfig := &logging.LogConfig{
        Filename:   "app.log",
        MaxSize:    10,    // 单位：MB
        MaxBackups: 5,     // 保留的旧日志文件数量
        MaxAge:     30,    // 保留的最大天数
        Compress:   true,  // 是否压缩
    }
    logConfig.SetDefault() // 设置默认值
    
    // 启用日志文件输出
    logging.EnableLogToFile(logConfig)
    
    logging.Infof("这条日志会被写入文件")
}
```

## 核心API

### 日志记录

- `Infof/Infoc`: 记录信息级别的日志
- `Debugf/Debugc`: 记录调试级别的日志
- `Warnf/Warnc`: 记录警告级别的日志
- `Errorf/Errorc`: 记录错误级别的日志
- `Fatalf/Fatalc`: 记录致命错误并退出程序
- `PanicError`: 如果错误不为nil，记录错误并触发panic

### 上下文操作

- `With(ctx, msg, v...)`: 向日志上下文添加分组或键值对

### 日志配置

- `SetLogger(logger)`: 设置自定义日志记录器
- `GetLogger()`: 获取当前日志记录器
- `Enable(level)`: 设置日志级别
- `SetOutput(writer)`: 设置日志输出目标
- `EnableLogToFile(config)`: 启用日志文件输出

### 实用工具

- `Jsonify(v)`: 将对象序列化为美化的JSON字符串
- `JsonifyNoIndent(v)`: 将对象序列化为无缩进的JSON字符串
- `TimeFuncDuration()`: 计算函数执行时间
- `TimeDurationDefer(prefix...)`: 用于defer的函数执行时间计算

## 日志级别

- `level.LevelDebug`: 调试级别 (-4)
- `level.LevelInfo`: 信息级别 (0)
- `level.LevelWarn`: 警告级别 (4)
- `level.LevelError`: 错误级别 (8)

## 高级用法

### 创建自定义日志格式

可以通过`slog`包创建自定义格式的日志记录器：

```go
logger := slog.NewSlogPrettyLogger(os.Stdout, 
    slog.WithCalldepth(6),
    slog.WithSource(),
    slog.WithHost(),
)
logging.SetLogger(logger)
```

### Kafka日志输出

```go
writer := &kafka.Writer{
    Addr:     kafka.TCP("localhost:9092"),
    Topic:    "logs",
    Balancer: &kafka.LeastBytes{},
}
logger := slog.NewSlogKafkaLogger(writer)
logging.SetLogger(logger)
```
