# Go Utils

本项目是一个 Go 语言工具库脚手架，包含了在开发过程中常用的各种功能模块。

## 模块概览

以下是 `go-utils` 包含的主要模块及其功能简介：

* [`cores`](#cores):  脚手架核心功能。
* [`example`](#example): 示例代码或用法演示。
* [`internal`](#internal): 项目内部使用的包，通常不建议外部直接引用。
* [`localrepos`](localrepos/README.md): 本地仓库或存储相关的工具。
* [`logging`](logging/README.md): 日志记录相关的工具。
* [`mysqlutils`](mysqlutils/README.md): MySQL 数据库操作相关的工具库。
* [`redisutils`](redisutils/README.md): Redis 操作相关的工具库。
* [`snail`](#snail):  延迟函数。
* [`flags`](flags/README.md): 命令行参数或配置项处理相关的工具。
* [`utils`](#utils): 通用工具函数集合。

## 使用示例

### `cores` - 基础服务框架

`cores` 提供了一个基础的服务启动和管理框架，集成了多种常用功能：

#### 主要特性

* **优雅退出**: 捕获系统信号并安全关闭所有资源
* **HTTP服务**: 内置HTTP服务器支持，可挂载自定义处理器
* **后台Worker**: 支持添加多个后台工作协程
* **Pprof支持**: 内置性能分析工具
* **链路追踪**: 支持Jaeger/OpenTelemetry链路追踪

#### 性能基准测试

以下是各框架性能基准测试结果（测试环境：Apple M4 Pro，darwin/arm64）：

| 框架 | 操作/秒 | 每操作耗时 | 每操作内存分配 | 每操作内存分配次数 |
|------|---------|------------|----------------|-------------------|
| NativeHTTP | ~27350 | ~40.5μs | ~8617 B | ~95 次 |
| Gin | ~28930 | ~41.5μs | ~9482 B | ~101 次 |
| Fiber | ~33670 | ~35.3μs | ~6547 B | ~74 次 |
| CoresWithHTTP | ~27810 | ~40.4μs | ~8579 B | ~94 次 |
| CoresWithGin | ~27600 | ~41.9μs | ~9442 B | ~100 次 |
| CoresWithFiber | ~25490 | ~48.9μs | ~11275 B | ~124 次 |

测试结果显示：

- 基于`cores`的框架与原生实现相比，性能损失较小，提供了更多功能的同时保持了较高效率
- `fiber` 与 `cores` 集成由于会将 `fiber.App` 通过适配层转换成 `http.Handler`，会丧失一些 `fasthttp` 的特性。考虑后面优化解决

#### 使用方法

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/miebyte/goutils/cores"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/flags"
)

var (
	port = flags.Int("port", 8080, "Service port")
)

func main() {
	flags.Parse()

	// 创建一个新的 Cores 服务实例
	srv := cores.NewCores(
		// 启用 pprof 性能分析接口
		cores.WithPprof(),
		// 配置HTTP处理器
		cores.WithHttpHandler("/api", myHttpHandler),
		// 启用跨域支持
		cores.WithHttpCORS(),
		// 添加一个后台 worker 任务
		cores.WithWorker(func(ctx context.Context) error {
			ticker := time.NewTicker(time.Second * 5)
			defer ticker.Stop()

			logging.Infof("Background worker started.")
			for {
				select {
				case <-ctx.Done(): // 监听退出信号
					logging.Infof("Background worker stopping: %v", ctx.Err())
					return ctx.Err()
				case t := <-ticker.C:
					fmt.Printf("Worker is running at %s\n", t.Format(time.RFC3339))
					// 在这里执行你的后台任务逻辑
				}
			}
		}),
		// 设置默认等待时间
		cores.WithDefaultMaxWait(time.Second * 10),
		// 等待所有worker完成后再退出
		cores.WithWaitAllDone(),
	)

	// 启动服务，监听指定端口
	// Start 会阻塞直到服务退出
	logging.Infof("Starting server on port %d", port())
	err := cores.Start(srv, port())
	if err != nil {
		logging.Panicf("Failed to start server: %v", err)
	}

	logging.Infof("Server stopped gracefully.")
}
```

### `flags` - 配置加载

详细文档请参阅 [flags/README.md](flags/README.md)。

```go
var (
	intFlag = flags.Int("port", 8080, "HTTP port")
)

func main() {
	flags.Parse()
	port := intFlag()
	// ...
}
```

### `logging` - 日志库

详细文档请参阅 [logging/README.md](logging/README.md)。

### `redisutils` - Redis客户端

详细文档请参阅 [redisutils/README.md](redisutils/README.md)。

### `mysqlutils` - MySQL客户端

详细文档请参阅 [mysqlutils/README.md](mysqlutils/README.md)。

### `localrepos` - 本地仓库

详细文档请参阅 [localrepos/README.md](localrepos/README.md)。
