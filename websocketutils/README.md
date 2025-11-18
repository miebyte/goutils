# websocketutils

`websocketutils` 是一个围绕 [gorilla/websocket](https://github.com/gorilla/websocket) 封装的轻量级工具库，提供命名空间（Namespace）、房间（Room）以及中间件能力，帮助你快速构建具备 Socket.io 风格语义的 Go WebSocket 服务端。

## 特性

- **基于路径的命名空间**：`Server` 会根据请求 URL 自动选择或创建命名空间，默认包含 `/` 命名空间。
- **房间广播**：连接可加入多个房间，支持命名空间广播、房间广播以及排除自身的发送模式。
- **多层中间件**：同时支持全局中间件与命名空间中间件，对连接做统一治理。
- **事件驱动**：命名空间级 `EventHandler` 与连接级 `MessageHandler` 共同组成事件体系，便于拆分业务逻辑。
- **内建心跳**：可通过 `WithHeartbeat` 配置 Ping/Pong，确保连接健康。

## 安装

```bash
go get github.com/miebyte/goutils/websocketutils
```

## 快速开始

```go
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/miebyte/goutils/websocketutils"
)

func main() {
	srv := websocketutils.NewServer(
		websocketutils.WithHeartbeat(30*time.Second, time.Minute),
	)

	srv.Use(func(s websocketutils.Socket) error {
		log.Printf("connected %s to %s", s.ID(), s.Namespace())
		return nil
	})

	orders := srv.Of("/orders")
	orders.On(websocketutils.EventConnection, func(socket websocketutils.Socket) {
		socket.On("join", func(s websocketutils.Socket, payload json.RawMessage) {
			var req struct {
				Room string `json:"room"`
			}
			if err := json.Unmarshal(payload, &req); err == nil {
				_ = s.Join(req.Room)
			}
		})
	})

	http.Handle("/orders", srv)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

## 示例

项目在 `example/` 下提供了一个简单的 Gin 服务端与命令式客户端：

```bash
# 启动服务端
go run ./example/server

# 启动客户端
go run ./example/client
```

客户端通过 HTTP 路由触发 `join`、`broadcast` 等事件，便于快速验证命名空间、房间以及广播行为。

## 核心概念

- **Server**：组合了 HTTP 协议升级、命名空间路由、全局中间件以及心跳配置。通过 `ServeHTTP` 自动根据请求路径选择命名空间，并为新连接绑定默认房间。
- **Namespace**：维护自身的中间件、事件处理器、房间和连接集合。支持 `On/Use/Emit/To` 等接口，以实现隔离的业务逻辑。
- **Room**：命名空间内部的连接集合，提供 `Emit` 与 `EmitExcept` 方法实现房间级广播。
- **Socket**：对外暴露的连接抽象，支持事件注册、发送消息、加入/离开房间、读取上下文等操作。
- **Middleware**：函数签名为 `func(Socket) error`，可用于鉴权、数据初始化等。

## API 速览

| 组件 | 方法 | 说明 |
| --- | --- | --- |
| `Server` | `NewServer(opts...)` | 创建具备默认 `/` 命名空间的服务实例 |
|  | `ServeHTTP` | 将 HTTP 连接升级并路由到命名空间 |
|  | `Use` | 注册全局中间件 |
|  | `Of` | 获取或创建命名空间 |
| `Namespace` | `On`、`Use`、`Emit`、`To`、`Room` | 命名空间级事件、广播、中间件以及房间管理 |
| `Socket` | `On`、`Emit`、`Join/Leave`、`Rooms`、`Close` | 连接级事件驱动与房间操作 |
| `Room` | `Emit`、`EmitExcept` | 针对房间内连接广播消息 |

更多签名可在 `types.go` 中的接口定义里找到。

## 错误与心跳

- 当尝试向不存在的房间广播或加入房间失败时，会返回 `ErrNoSuchRoom`。
- 写入已关闭的连接会返回 `ErrConnClosed`。
- 通过 `WithHeartbeat(pingInterval, pongTimeout)` 可以配置服务端定期发送 Ping（默认 25s）并设置 ReadDeadline（默认 60s），从而及时回收异常连接。

## 开发

- 运行 `go test ./...` 可执行基础单元测试（若存在）。
- 代码遵循 `go fmt`，在提交前可执行 `gofmt -w .` 保持风格一致。

## 目录

```
.
├── conn.go          # Socket 抽象与心跳、读写循环
├── namespace.go     # 命名空间生命周期与房间广播
├── room.go          # 房间管理
├── server.go        # Server 入口与 HTTP 适配
├── types.go         # 对外接口与公共类型
└── example/         # server/client 示例
```

