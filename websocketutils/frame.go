package websocketutils

import "encoding/json"

// Frame 是客户端与服务端之间的统一事件消息格式。
type Frame struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data,omitempty"`
}
