# websocketutils

`websocketutils` 提供一套基于 Gorilla WebSocket 的高层封装，简化 WebSocket 服务端与客户端的事件驱动编程。通过命名空间与房间模型、事件广播与 Ack 机制、心跳与超时管理，让构建可扩展的实时应用更加直接。

## 特性

- 事件驱动：支持事件注册、广播、Ack 应答，便于实现 RPC 风格的实时通信。
- 命名空间与房间：可对连接进行逻辑分组，实现多频道、多房间隔离。
- 可配置能力：支持连接钩子、读写超时、心跳间隔、跨域策略、缓冲区等自定义。
- 并发安全：内部使用锁保护状态，适配高并发场景。
- 示例完备：提供服务端与客户端示例，帮助快速验证与调试。

## 安装

```bash
go get github.com/miebyte/goutils/websocketutils
```

## 快速开始

### 服务端

```go
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/ginutils"
	"github.com/miebyte/goutils/websocketutils"
)

func main() {
	engine := ginutils.Default()
	ws := websocketutils.NewServer(
		websocketutils.WithPingInterval(2*time.Second),
	)

	ws.On("chat", func(client *websocketutils.Client, event *websocketutils.Event) {
		var payload map[string]string
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			_ = client.Emit("error", map[string]any{"error": "invalid payload"})
			return
		}
		_ = ws.EmitExcept("chat", payload, client)
		if event.Ack != nil {
			_ = event.Ack(map[string]any{"status": "ok"})
		}
	})

	engine.GET("/ws", func(c *gin.Context) {
		ws.ServeHTTP(c.Writer, c.Request)
	})

	if err := engine.Run(":8080"); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server start failed: %v", err)
	}
}
```

### 客户端

```go
package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	conn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:8080/ws", http.Header{})
	if err != nil {
		log.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	writeMu := &sync.Mutex{}
	go readLoop(conn)

	if err := sendJSON(conn, writeMu, map[string]any{
		"type":      "event",
		"namespace": "/",
		"event":     "chat",
		"data": map[string]any{
+			"user":    "example-client",
+			"message": "hello world",
		},
	}); err != nil {
		log.Printf("send failed: %v", err)
	}
	time.Sleep(5 * time.Second)
}

func readLoop(conn *websocket.Conn) {
	for {
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("read error: %v", err)
			return
		}
		log.Printf("recv %#v", msg)
	}
}

func sendJSON(conn *websocket.Conn, mu *sync.Mutex, v any) error {
	mu.Lock()
	defer mu.Unlock()
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return conn.WriteJSON(v)
}
```

## 核心概念

- **Server**：对外提供 WebSocket 服务，负责升级连接、维护命名空间与房间、广播事件。
- **Client**：服务端对单个连接的抽象，支持发送事件、加入/退出命名空间或房间、等待 Ack。
- **Namespace**：命名空间隔离不同业务频道，每个命名空间内可注册事件处理器。
- **Room**：命名空间下的房间，用于更细粒度的广播。
- **Event**：事件回调的载体，包含事件名称、原始数据以及可选 Ack 函数。

## 主要 API

### Server 构造与 Option

- `NewServer(opts ...Option)`：创建新的服务实例。
- `WithOrigins`：限制跨域 Origin。
- `WithBufferSize`：调整读写缓冲区大小。
- `WithHooks`：注册连接建立与关闭的钩子。
- `WithTimeout`：自定义读写超时。
- `WithPingInterval`：配置心跳发送间隔。

### Server 方法

- `ServeHTTP`：接入 `net/http` 体系的升级入口。
- `On`：在默认命名空间注册事件处理器。
- `Namespace`：获取或创建命名空间并注册事件。
- `Emit/EmitExcept`：向命名空间广播事件，可排除特定客户端。
- `EmitToRoom/EmitToRoomExcept`：向房间广播事件。

### Client 能力

- `Emit/EmitToNamespace`：向服务端发送事件。
- `EmitWithAck`：发送事件并等待服务端 `Ack`，支持超时控制。
- `JoinNamespace/LeaveNamespace`：加入或退出命名空间。
- `JoinRoom/LeaveRoom`：加入或退出房间。
- `Close`：主动关闭连接。

## 超时与心跳

- 默认写超时 `10s`，读超时 `30s`，Ack 超时 `5s`。
- 服务端会以 `pingInterval` 周期发送 Ping，客户端需处理 Pong 以保持连接存活。
- 通过 Option 可根据业务需求调整相关参数。

## 示例运行

```bash
# 启动示例服务端（需要 Gin）
cd example/server
go run .

# 在新的终端启动示例客户端
cd ../client
go run .
```

示例客户端会加入默认命名空间与房间，周期性发送聊天消息并接收服务端广播与 Ack。

## 开发建议

- 推荐使用 Go 1.21+（项目 `go.mod` 指定 1.25.4）。
- 事件处理函数请保证快速返回，避免阻塞广播循环；如需长耗时操作，请自行开 goroutine。
- Ack 机制用于请求-响应式事件交互，注意合理设置上下文超时。
- 当需要自定义序列化格式，可在事件处理器中结合 `json.RawMessage` 自行解码。
- 对外暴露的 HTTP 路由应结合身份鉴权与 Origin 校验，保证连接安全。

