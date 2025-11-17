package websocketutils

import (
	"encoding/json"
	"time"
)

const (
	defaultNamespace    = "/"
	defaultWriteTimeout = 10 * time.Second
	defaultReadTimeout  = 30 * time.Second
	defaultAckTimeout   = 5 * time.Second
)

// 消息类型定义
const (
	MessageTypePing  = "ping"
	MessageTypePong  = "pong"
	MessageTypeEvent = "event"
	MessageTypeJoin  = "join"
	MessageTypeLeave = "leave"
	MessageTypeAck   = "ack"
	MessageTypeError = "error"
)

// Option 配置 Server 行为
type Option func(*Server)

// EventHandler 处理事件消息
type EventHandler func(*Client, *Event)

// Event 表示收到的事件消息
type Event struct {
	Namespace string
	Name      string
	Data      json.RawMessage
	Ack       func(any) error
}
