package websocketutils

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/miebyte/goutils/logging"
)

// Client 表示单个 websocket 会话
type Client struct {
	server *Server
	conn   *websocket.Conn

	send chan []byte

	idCounter atomic.Uint64

	ackMu    sync.Mutex
	ackWaits map[string]chan json.RawMessage

	roomsMu sync.RWMutex
	rooms   map[string]struct{}

	namespacesMu sync.RWMutex
	namespaces   map[string]struct{}

	closeOnce sync.Once
	closed    chan struct{}
}

func newClient(server *Server, conn *websocket.Conn) *Client {
	c := &Client{
		server:     server,
		conn:       conn,
		send:       make(chan []byte, 64),
		ackWaits:   make(map[string]chan json.RawMessage),
		rooms:      make(map[string]struct{}),
		namespaces: make(map[string]struct{}),
		closed:     make(chan struct{}),
	}
	conn.SetReadLimit(1 << 20)
	return c
}

// ID 生成客户端唯一标识
func (c *Client) ID() string {
	return c.conn.RemoteAddr().String()
}

// Emit 向默认命名空间发送事件
func (c *Client) Emit(event string, payload any) error {
	return c.EmitToNamespace(defaultNamespace, event, payload)
}

// EmitToNamespace 向指定命名空间发送事件
func (c *Client) EmitToNamespace(namespace, event string, payload any) error {
	msg, err := encodeMessage(MessageTypeEvent, namespace, event, payload, "")
	if err != nil {
		return err
	}
	return c.enqueue(msg)
}

// EmitWithAck 发送默认命名空间事件并等待 ack
func (c *Client) EmitWithAck(ctx context.Context, event string, payload any) (json.RawMessage, error) {
	return c.EmitWithAckNamespace(ctx, defaultNamespace, event, payload)
}

// EmitWithAckNamespace 发送命名空间事件并等待 ack
func (c *Client) EmitWithAckNamespace(ctx context.Context, namespace, event string, payload any) (json.RawMessage, error) {
	ackID := c.nextAckID()
	msg, err := encodeMessage(MessageTypeEvent, namespace, event, payload, ackID)
	if err != nil {
		return nil, err
	}
	wait := make(chan json.RawMessage, 1)
	c.registerAck(ackID, wait)
	defer c.removeAck(ackID)
	if err := c.enqueue(msg); err != nil {
		return nil, err
	}
	select {
	case data := <-wait:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(defaultAckTimeout):
		return nil, errors.New("ack timeout")
	case <-c.closed:
		return nil, errors.New("connection closed")
	}
}

// JoinNamespace 加入命名空间
func (c *Client) JoinNamespace(namespace string) {
	if namespace == "" {
		namespace = defaultNamespace
	}
	c.server.joinNamespace(c, namespace)
}

// LeaveNamespace 离开命名空间
func (c *Client) LeaveNamespace(namespace string) {
	if namespace == "" {
		namespace = defaultNamespace
	}
	c.server.leaveNamespace(c, namespace)
}

// Join 加入默认命名空间的房间
func (c *Client) Join(room string) {
	c.JoinRoom(defaultNamespace, room)
}

// JoinRoom 加入指定命名空间房间
func (c *Client) JoinRoom(namespace, room string) {
	if namespace == "" {
		namespace = defaultNamespace
	}
	if room == "" {
		return
	}
	c.server.joinRoom(namespace, c, room)
}

// Leave 离开默认命名空间房间
func (c *Client) Leave(room string) {
	c.LeaveRoom(defaultNamespace, room)
}

// LeaveRoom 离开指定命名空间房间
func (c *Client) LeaveRoom(namespace, room string) {
	if namespace == "" {
		namespace = defaultNamespace
	}
	if room == "" {
		return
	}
	c.server.leaveRoom(namespace, c, room)
}

// Close 主动关闭连接
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closed)
		close(c.send)
		c.conn.Close()
		c.server.unregisterClient(c)
	})
}

func (c *Client) enqueue(msg []byte) error {
	select {
	case <-c.closed:
		return errors.New("connection closed")
	default:
	}
	select {
	case c.send <- msg:
		return nil
	default:
		return errors.New("send buffer full")
	}
}

func (c *Client) readLoop() {
	defer c.Close()
	c.conn.SetReadDeadline(time.Now().Add(c.server.readTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.server.readTimeout))
		return nil
	})
	for {
		var msg WireMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			logging.Warnf("readLoop error: %v", err)
			return
		}
		ns := msg.Namespace
		if ns == "" {
			ns = defaultNamespace
		}
		switch msg.Type {
		case MessageTypePing:
			resp, _ := encodeMessage(MessageTypePong, ns, "", nil, "")
			c.enqueue(resp)
		case MessageTypePong:
			continue
		case MessageTypeEvent:
			c.server.dispatchEvent(ns, c, &msg)
		case MessageTypeJoin:
			if msg.Room == "" {
				c.JoinNamespace(ns)
			} else {
				c.JoinRoom(ns, msg.Room)
			}
		case MessageTypeLeave:
			if msg.Room == "" {
				c.LeaveNamespace(ns)
			} else {
				c.LeaveRoom(ns, msg.Room)
			}
		case MessageTypeAck:
			c.resolveAck(msg.AckID, msg.Data)
		default:
			errResp := WireMessage{Type: MessageTypeError, Namespace: ns, Error: "unknown message type"}
			b, _ := json.Marshal(errResp)
			c.enqueue(b)
		}
	}
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(c.server.pingInterval)
	defer func() {
		ticker.Stop()
		c.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(c.server.writeTimeout))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(c.server.writeTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		case <-c.closed:
			return
		}
	}
}

func (c *Client) nextAckID() string {
	id := c.idCounter.Add(1)
	return c.ID() + "#" + strconv.FormatUint(id, 10)
}

func (c *Client) registerAck(id string, ch chan json.RawMessage) {
	c.ackMu.Lock()
	c.ackWaits[id] = ch
	c.ackMu.Unlock()
}

func (c *Client) removeAck(id string) {
	c.ackMu.Lock()
	delete(c.ackWaits, id)
	c.ackMu.Unlock()
}

func (c *Client) resolveAck(id string, data json.RawMessage) {
	if id == "" {
		return
	}
	c.ackMu.Lock()
	ch, ok := c.ackWaits[id]
	if ok {
		delete(c.ackWaits, id)
	}
	c.ackMu.Unlock()
	if ok {
		select {
		case ch <- data:
		default:
		}
	}
}

func (c *Client) addNamespace(namespace string) {
	c.namespacesMu.Lock()
	c.namespaces[namespace] = struct{}{}
	c.namespacesMu.Unlock()
}

func (c *Client) removeNamespace(namespace string) {
	c.namespacesMu.Lock()
	delete(c.namespaces, namespace)
	c.namespacesMu.Unlock()
	c.roomsMu.Lock()
	for key := range c.rooms {
		if hasNamespacePrefix(key, namespace) {
			delete(c.rooms, key)
		}
	}
	c.roomsMu.Unlock()
}

func (c *Client) inNamespace(namespace string) bool {
	c.namespacesMu.RLock()
	_, ok := c.namespaces[namespace]
	c.namespacesMu.RUnlock()
	return ok
}

func (c *Client) addRoom(namespace, room string) {
	key := roomKey(namespace, room)
	c.roomsMu.Lock()
	c.rooms[key] = struct{}{}
	c.roomsMu.Unlock()
}

func (c *Client) removeRoom(namespace, room string) {
	key := roomKey(namespace, room)
	c.roomsMu.Lock()
	delete(c.rooms, key)
	c.roomsMu.Unlock()
}

func roomKey(namespace, room string) string {
	return namespace + "::" + room
}

func hasNamespacePrefix(key, namespace string) bool {
	return len(key) >= len(namespace)+2 && key[:len(namespace)] == namespace && key[len(namespace):len(namespace)+2] == "::"
}
