package websocketutils

import (
	"encoding/json"

	"github.com/gorilla/websocket"
)

type wsTransport struct {
	conn *websocket.Conn
}

func newWSTransport(conn *websocket.Conn) Transport {
	return &wsTransport{conn: conn}
}

func (t *wsTransport) Read() (*Frame, error) {
	_, data, err := t.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	var frame Frame
	if err := json.Unmarshal(data, &frame); err != nil {
		return nil, err
	}

	return &frame, nil
}

func (t *wsTransport) Write(messageType int, data []byte) error {
	return t.conn.WriteMessage(messageType, data)
}

func (t *wsTransport) Close() error {
	return t.conn.Close()
}

var _ Transport = (*wsTransport)(nil)
