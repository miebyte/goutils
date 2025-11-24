package websocketutils

import (
	"context"
	"net/http"
)

// EventHandler 定义命名空间级别的事件处理器。
type EventHandler func(*Context)

// RoomAPI 抽象 Room 能力。
type RoomAPI interface {
	// Name 返回房间名称。
	Name() string
	// Broadcast 广播事件给房间成员。
	Broadcast(event string, data any)
	// Add 将连接加入房间。
	Add(conn Conn)
	// Remove 将连接移出房间。
	Remove(conn Conn)
	// Members 返回房间成员列表。
	Members() []Conn
}

// NamespaceAPI
type NamespaceAPI interface {
	// Namespace 返回命名空间名称。
	Name() string
	// Emit 广播事件到整个命名空间。
	Emit(event string, payload any) error
	// EmitExcept 广播事件到整个命名空间，除了指定连接。
	EmitExcept(event string, payload any, conn Conn) error
	// On 绑定事件处理函数。
	On(event string, handler EventHandler)
	// Room 返回一个房间。
	Room(name string) RoomAPI
	// To 返回房间上下文，用于多房间广播。
	To(room string) *RoomContext
}

// ServerAPI 抽象命名空间能力与 http.Handler。
// 它是一个最底层的抽象，它对外暴露了 NamespaceAPI 能力，以及自己的 id 和房间管理能力。
// 当连接进来时，会根据连接的 uri 路径自动加入到对应的命名空间。
// 比如：ws://localhost:8080/namespace1。那么这个连接就会自动加入到 namespace1 命名空间。
// 同时，还会在当前命名空间下加入到以自身 id 为名称的房间。
type ServerAPI interface {
	Of(name string) NamespaceAPI
	// Broadcast 广播事件到所有命名空间。
	Broadcast(event string, payload any)
	// ServeHTTP 实现 http.Handler。
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// Conn 代表一个客户端连接。
type Conn interface {
	// ID 返回连接 ID。
	ID() string
	// Request 返回握手请求。
	Request() *http.Request
	// Close 关闭连接。
	Close() error
	// Namespace 返回当前连接加入的命名空间。
	Namespace() NamespaceAPI
	// SendFrame 发送数据帧。
	SendFrame([]byte) error
	// Emit 发送事件给当前连接。
	Emit(event string, payload any) error
	// On 绑定事件处理函数。
	On(event string, handler EventHandler)
	// Join 将连接加入房间。
	Join(room string) error
	// Leave 将连接移出房间。
	Leave(room string)
	// Rooms 返回当前连接所在的房间列表。
	Rooms() []string
	// Context 返回连接上下文。
	Context() context.Context
}

// Transport 定义底层连接传输接口
type Transport interface {
	// Read 读取数据帧
	Read() (*Frame, error)
	// Write 写入数据
	Write(messageType int, data []byte) error
	// Close 关闭连接
	Close() error
}

type AllowRequestFunc func(*http.Request) (*http.Request, error)
