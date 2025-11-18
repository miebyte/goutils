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
	// EventJoinRoom 允许客户端申请加入房间。
	EventJoinRoom = "JoinRoom"
	// EventLeaveRoom 允许客户端退出房间。
	EventLeaveRoom = "LeaveRoom"
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

// NamespaceAPI 抽象 On/Use/Emit 能力。
type NamespaceAPI interface {
	On(string, EventHandler)
	Use(Middleware)
	Emit(string, any) error
}

// ServerAPI 抽象命名空间能力与 http.Handler。
type ServerAPI interface {
	NamespaceAPI
	Of(string) *Namespace
	ServeHTTP(http.ResponseWriter, *http.Request)
}

// Socket 为对外暴露的连接能力。
type Socket interface {
	ID() string
	Namespace() string
	Context() context.Context
	On(string, MessageHandler)
	Emit(string, any) error
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
