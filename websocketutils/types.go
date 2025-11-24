package websocketutils

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

const (
	// EventConnection 是命名空间连接事件。
	EventConnection = "connection"
	// EventDisconnect 是连接断开事件。
	EventDisconnect = "disconnect"
)

// ErrConnClosed 表示连接已经关闭。
var ErrConnClosed = errors.New("websocketutils: connection closed")

// ErrNoSuchRoom 表示目标房间不存在。
var ErrNoSuchRoom = errors.New("websocketutils: room not found")

// Frame 是客户端与服务端之间的统一事件消息格式。
type Frame struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// Middleware 定义连接建立后的中间件。
type Middleware func(Conn) error

// EventHandler 定义命名空间级别的事件处理器。
type EventHandler func(Conn)

// ErrorHandler 定义错误处理器
type ErrorHandler func(Conn, error)

// Emitter 定义广播器。
type Emitter interface {
	Emit(string, any) error
}

// TargetEmitter 支持链式房间广播。
type TargetEmitter interface {
	Emitter
	EmitExcept(event string, payload any, socket Conn) error
	To(...string) TargetEmitter
}

// RoomAPI 抽象 Room 能力。
type RoomAPI interface {
	Name() string

	// 广播给房间成员
	Broadcast(event string, data any)

	// 管理连接
	Add(conn Conn)
	Remove(conn Conn)
	Members() []Conn
}

// NamespaceAPI
type NamespaceAPI interface {
	// Emit 广播事件到整个命名空间。
	Emitter
	// Namespace 返回命名空间名称。
	Name() string
	// On 绑定事件处理函数。
	On(string, EventHandler)
	// OnError 绑定错误处理函数
	OnError(ErrorHandler)
	// Use 增加中间件。
	Use(Middleware)
	// Room 返回一个房间。
	Room(name string) RoomAPI
	// To 返回房间广播器。
	To(...string) TargetEmitter
}

// ServerAPI 抽象命名空间能力与 http.Handler。
type ServerAPI interface {
	Of(string) *Namespace
	// 广播（所有 namespace）
	Broadcast(event string, data any)
	// 中间件
	Use(middleware Middleware)
	ServeHTTP(http.ResponseWriter, *http.Request)
}

// Conn 代表一个客户端连接，可收发事件，加入/退出房间。。
// 它是一个最底层的抽象，它对外暴露了 NamespaceAPI 能力，以及自己的 id 和房间管理能力。
// 它内部维护了一个命名空间对象的引用，以及一个房间管理器。
// 一个 Socket 仅属于一个命名空间，但是可以加入其中的多个房间。
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
	Emit(string, any) error
	// On 绑定事件处理函数。
	On(string, EventHandler)
	// Use 增加中间件。
	Use(Middleware)
	// Join 将连接加入房间。
	Join(string) error
	// Leave 将连接移出房间。
	Leave(string)
	// Rooms 返回当前连接所在的房间列表。
	Rooms() []string
	Context() context.Context
	Set(key string, value any)
	Get(key string) (any, bool)
}

// Transport 定义底层连接传输接口
type Transport interface {
	// Read 读取数据帧
	Read() (*Frame, error)
	// Write 写入数据
	Write(messageType int, data []byte) error
	// WriteControl 写入控制消息
	WriteControl(messageType int, data []byte, deadline time.Time) error
	// SetReadDeadline 设置读取超时
	SetReadDeadline(t time.Time) error
	// SetPongHandler 设置 Pong 处理器
	SetPongHandler(h func(appData string) error)
	// Close 关闭连接
	Close() error
}

func encodeFrame(event string, payload any) ([]byte, error) {
	frame := Frame{Event: event}
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		frame.Data = body
	}
	return json.Marshal(frame)
}
