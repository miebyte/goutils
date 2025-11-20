package websocketutils

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/miebyte/goutils/logging"
)

const pingWriteWait = 5 * time.Second

// Conn 封装 WebSocket 连接并提供 Socket 能力。
type Conn struct {
	id        string
	namespace *Namespace
	ws        *websocket.Conn
	req       *http.Request

	send      chan []byte
	handlers  map[string][]MessageHandler
	rooms     map[string]struct{}
	mu        sync.RWMutex
	closeOnce sync.Once
	closed    chan struct{}

	ctx    context.Context
	cancel context.CancelFunc

	pingInterval time.Duration
	pongTimeout  time.Duration
}

func newConn(ctx context.Context, ns *Namespace, ws *websocket.Conn, req *http.Request) *Conn {
	id := nextConnID()
	ctx, cancel := context.WithCancel(ctx)
	ctx = logging.With(ctx, "ConnID", id)
	pingInterval, pongTimeout := ns.server.heartbeatConfig()
	c := &Conn{
		id:           id,
		namespace:    ns,
		ws:           ws,
		req:          req,
		send:         make(chan []byte, 16),
		handlers:     make(map[string][]MessageHandler),
		rooms:        make(map[string]struct{}),
		closed:       make(chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
		pingInterval: pingInterval,
		pongTimeout:  pongTimeout,
	}

	c.setupHeartbeat()
	go c.readLoop()
	go c.writeLoop()
	return c
}

// ID 返回连接 ID。
func (c *Conn) ID() string {
	return c.id
}

// Namespace 返回命名空间名称。
func (c *Conn) Namespace() string {
	return c.namespace.name
}

// Request 返回握手请求。
func (c *Conn) Request() *http.Request {
	return c.req
}

// Context 返回连接上下文。
func (c *Conn) Context() context.Context {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ctx
}

func (c *Conn) SetContext(ctx context.Context) {
	c.mu.Lock()
	c.ctx = ctx
	c.mu.Unlock()
}

// On 绑定事件处理函数。
func (c *Conn) On(event string, handler MessageHandler) {
	if event == "" || handler == nil {
		return
	}
	c.mu.Lock()
	c.handlers[event] = append(c.handlers[event], handler)
	c.mu.Unlock()
}

// Emit 发送事件。
func (c *Conn) Emit(event string, payload any) error {
	frame, err := encodeFrame(event, payload)
	if err != nil {
		return err
	}
	return c.sendFrame(frame)
}

// Join 将连接加入房间。
func (c *Conn) Join(room string) error {
	if room == "" {
		return ErrNoSuchRoom
	}
	return c.namespace.joinRoom(room, c)
}

// Leave 将连接移出房间。
func (c *Conn) Leave(room string) {
	if room == "" {
		return
	}
	c.namespace.leaveRoom(room, c)
}

// Rooms 返回当前连接所在的房间列表。
func (c *Conn) Rooms() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rooms := make([]string, 0, len(c.rooms))
	for name := range c.rooms {
		rooms = append(rooms, name)
	}
	return rooms
}

// Close 关闭连接。
func (c *Conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closed)
		c.cancel()
		c.namespace.detachConnection(c)

		rooms := c.Rooms()
		for _, room := range rooms {
			c.namespace.leaveRoom(room, c)
		}

		err = c.ws.Close()
	})
	return err
}

func (c *Conn) readLoop() {
	for {
		var frame Frame
		if err := c.ws.ReadJSON(&frame); err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				websocketLogger.Errorc(c.ctx, "read json error: %v", err)
			} else {
				websocketLogger.Infoc(c.ctx, "connection closed")
			}
			break
		}

		if frame.Event == "" {
			websocketLogger.Warnc(c.ctx, "read empty event")
			continue
		}

		websocketLogger.Debugc(c.ctx, "read event=%s", logging.JsonifyNoIndent(frame))
		c.dispatch(frame)
	}
	_ = c.Close()
}

func (c *Conn) writeLoop() {
	var (
		ticker *time.Ticker
		pingC  <-chan time.Time
	)
	if c.pingInterval > 0 {
		ticker = time.NewTicker(c.pingInterval)
		pingC = ticker.C
		defer ticker.Stop()
	}
	for {
		select {
		case <-c.closed:
			return
		case payload, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, payload); err != nil {
				_ = c.Close()
				return
			}
		case <-pingC:
			if err := c.sendPing(); err != nil {
				websocketLogger.Warnc(c.ctx, "send ping error: %v", err)
				_ = c.Close()
				return
			}
		}
	}
}

func (c *Conn) dispatch(frame Frame) {
	c.mu.RLock()
	handlers := append([]MessageHandler(nil), c.handlers[frame.Event]...)
	c.mu.RUnlock()
	if len(handlers) == 0 {
		return
	}
	for _, handler := range handlers {
		go func(h MessageHandler) {
			defer func() {
				if r := recover(); r != nil {
					websocketLogger.Errorc(c.ctx, "handler panic: %v", r)
				}
			}()
			h(c, frame.Data)
		}(handler)
	}
}

func (c *Conn) sendFrame(data []byte) error {
	select {
	case <-c.closed:
		return ErrConnClosed
	case c.send <- data:
		return nil
	default:
		return errors.New("websocketutils: connection buffer full")
	}
}

func (c *Conn) addRoom(name string) {
	c.mu.Lock()
	c.rooms[name] = struct{}{}
	c.mu.Unlock()
}

func (c *Conn) removeRoom(name string) {
	c.mu.Lock()
	delete(c.rooms, name)
	c.mu.Unlock()
}

func (c *Conn) setupHeartbeat() {
	if c.pongTimeout <= 0 {
		return
	}
	if err := c.refreshReadDeadline(); err != nil {
		websocketLogger.Warnc(c.ctx, "set read deadline error: %v", err)
	}
	c.ws.SetPongHandler(func(string) error {
		if err := c.refreshReadDeadline(); err != nil {
			websocketLogger.Warnc(c.ctx, "refresh read deadline error: %v", err)
			return err
		}
		return nil
	})
}

func (c *Conn) refreshReadDeadline() error {
	if c.pongTimeout <= 0 {
		return nil
	}
	return c.ws.SetReadDeadline(time.Now().Add(c.pongTimeout))
}

func (c *Conn) sendPing() error {
	if c.pingInterval <= 0 {
		return nil
	}
	websocketLogger.Debugc(c.ctx, "sending ping")
	return c.ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(pingWriteWait))
}

func nextConnID() string {
	return uuid.NewString()
}
