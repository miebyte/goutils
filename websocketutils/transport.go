package websocketutils

import (
	"time"

	"github.com/gorilla/websocket"
)

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
