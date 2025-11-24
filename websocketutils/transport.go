package websocketutils

import (
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketTransport 基于 gorilla/websocket 的传输实现
type WebSocketTransport struct {
	conn *websocket.Conn
}

func (t *WebSocketTransport) Read() (*Frame, error) {
	var frame Frame
	if err := t.conn.ReadJSON(&frame); err != nil {
		return nil, err
	}
	return &frame, nil
}

func (t *WebSocketTransport) Write(messageType int, data []byte) error {
	return t.conn.WriteMessage(messageType, data)
}

func (t *WebSocketTransport) WriteControl(messageType int, data []byte, deadline time.Time) error {
	return t.conn.WriteControl(messageType, data, deadline)
}

func (t *WebSocketTransport) SetReadDeadline(deadline time.Time) error {
	return t.conn.SetReadDeadline(deadline)
}

func (t *WebSocketTransport) SetPongHandler(h func(appData string) error) {
	t.conn.SetPongHandler(h)
}

func (t *WebSocketTransport) Close() error {
	return t.conn.Close()
}
