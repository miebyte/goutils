package websocketutils

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

const (
	// EventConnection 是命名空间连接事件。
	EventConnection = "connection"
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
type Middleware func(Socket) error

// EventHandler 定义命名空间级别的事件处理器。
type EventHandler func(Socket)

// MessageHandler 定义连接级别的事件处理器。
type MessageHandler func(Socket, json.RawMessage)

// Emitter 定义广播器。
type Emitter interface {
	Emit(string, any) error
}

// TargetEmitter 支持链式房间广播。
type TargetEmitter interface {
	Emitter
	To(string) TargetEmitter
}

// NamespaceAPI 抽象 On/Use/Emit 能力。
type NamespaceAPI interface {
	// Emit 广播事件到整个命名空间。
	Emitter
	// On 绑定事件处理函数。
	On(string, EventHandler)
	// Use 增加中间件。
	Use(Middleware)
	// To 返回房间广播器。
	To(string) TargetEmitter
}

// ServerAPI 抽象命名空间能力与 http.Handler。
type ServerAPI interface {
	NamespaceAPI
	// Of 返回一个命名空间
	Of(string) *Namespace
	ServeHTTP(http.ResponseWriter, *http.Request)
}

// Socket 为对外暴露的连接能力。
type Socket interface {
	Emitter
	ID() string
	Namespace() string
	Context() context.Context
	Request() *http.Request
	On(string, MessageHandler)
	Join(string) error
	Leave(string)
	Rooms() []string
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
