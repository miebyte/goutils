package websocketutils

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var _ Conn = (*socketConn)(nil)

type outboundMessage struct {
	messageType int
	payload     []byte
}

// socketConn 表示单个 websocket 连接。
// 一个 socketConn 仅属于一个命名空间，但是可以加入其中的多个房间。
type socketConn struct {
	id           string
	namespace    *namespace
	transport    Transport
	request      *http.Request
	pingInterval time.Duration
	pingTimeout  time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	send   chan outboundMessage
	pongCh chan struct{}

	handlersMu sync.RWMutex
	handlers   map[string][]EventHandler

	roomsMu sync.RWMutex
	rooms   map[string]struct{}

	closeOnce sync.Once
	closed    chan struct{}
	wg        sync.WaitGroup
}

func newSocketConn(ns *namespace, transport Transport, req *http.Request, queueSize int, pingInterval, pingTimeout time.Duration) *socketConn {
	parent := context.Background()
	if req != nil {
		if ctx := req.Context(); ctx != nil {
			parent = ctx
		}
	}
	baseCtx, cancel := context.WithCancel(parent)
	if queueSize <= 0 {
		queueSize = defaultSendQueueSize
	}
	return &socketConn{
		id:           uuid.NewString(),
		namespace:    ns,
		transport:    transport,
		request:      req,
		ctx:          baseCtx,
		cancel:       cancel,
		send:         make(chan outboundMessage, queueSize),
		handlers:     make(map[string][]EventHandler),
		rooms:        make(map[string]struct{}),
		closed:       make(chan struct{}),
		pingInterval: pingInterval,
		pingTimeout:  pingTimeout,
		pongCh:       make(chan struct{}, 1),
	}
}

func (c *socketConn) ID() string {
	return c.id
}

func (c *socketConn) Request() *http.Request {
	return c.request
}

func (c *socketConn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		c.cancel()
		close(c.closed)
		c.leaveAllRooms()
		if c.namespace != nil {
			c.namespace.removeConn(c.id)
			c.namespace.dispatch(EventDisconnect, c, nil)
		}
		close(c.send)
		if c.transport != nil {
			if closeErr := c.transport.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}
	})
	return err
}

func (c *socketConn) Namespace() NamespaceAPI {
	return c.namespace
}

func (c *socketConn) SendFrame(data []byte) error {
	return c.enqueue(outboundMessage{
		messageType: websocket.TextMessage,
		payload:     data,
	})
}

func (c *socketConn) Emit(event string, payload any) error {
	if event == "" {
		return nil
	}
	frame := Frame{Event: event}
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		frame.Data = raw
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	return c.enqueue(outboundMessage{
		messageType: websocket.TextMessage,
		payload:     data,
	})
}

func (c *socketConn) On(event string, handler EventHandler) {
	if event == "" || handler == nil {
		return
	}
	c.handlersMu.Lock()
	c.handlers[event] = append(c.handlers[event], handler)
	c.handlersMu.Unlock()
}

func (c *socketConn) Join(room string) error {
	if room == "" {
		return nil
	}
	c.roomsMu.Lock()
	if _, exists := c.rooms[room]; exists {
		c.roomsMu.Unlock()
		return nil
	}
	c.rooms[room] = struct{}{}
	c.roomsMu.Unlock()
	if rm := c.namespace.Room(room); rm != nil {
		rm.Add(c)
	}
	return nil
}

func (c *socketConn) Leave(room string) error {
	if room == "" {
		return nil
	}
	c.roomsMu.Lock()
	if _, exists := c.rooms[room]; !exists {
		c.roomsMu.Unlock()
		return nil
	}
	delete(c.rooms, room)
	c.roomsMu.Unlock()
	if rm := c.namespace.getRoom(room); rm != nil {
		rm.Remove(c)
	}

	return nil
}

func (c *socketConn) Rooms() []string {
	c.roomsMu.RLock()
	defer c.roomsMu.RUnlock()
	if len(c.rooms) == 0 {
		return nil
	}
	res := make([]string, 0, len(c.rooms))
	for room := range c.rooms {
		res = append(res, room)
	}
	return res
}

func (c *socketConn) Context() context.Context {
	return c.ctx
}

func (c *socketConn) run() {
	c.wg.Add(1)
	go c.writeLoop()
	if c.pingInterval > 0 && c.pingTimeout > 0 {
		c.wg.Add(1)
		go c.heartbeatLoop()
	}
	c.readLoop()
	c.Close()
	c.wg.Wait()
}

func (c *socketConn) readLoop() {
	for {
		frame, err := c.transport.Read()
		if err != nil {
			Logger().Warnf("conn read loop stopped conn=%s err=%v", c.id, err)
			return
		}
		if frame == nil || frame.Event == "" {
			continue
		}
		c.handleFrame(frame)
	}
}

func (c *socketConn) writeLoop() {
	defer c.wg.Done()
	for msg := range c.send {
		if err := c.transport.Write(msg.messageType, msg.payload); err != nil {
			Logger().Warnf("conn write loop stopped conn=%s err=%v", c.id, err)
			return
		}
	}
}

func (c *socketConn) handleFrame(frame *Frame) {
	if c.handleBuiltinEvent(frame) {
		return
	}
	ctx := newEventContext(c.ctx, c, c.namespace, frame.Event, frame.Data)

	if handlers := c.connHandlers(frame.Event); len(handlers) > 0 {
		for _, h := range handlers {
			callHandlerSafely(h, ctx)
		}
		return
	}
	c.namespace.dispatch(frame.Event, c, frame.Data)
}

func (c *socketConn) connHandlers(event string) []EventHandler {
	c.handlersMu.RLock()
	defer c.handlersMu.RUnlock()
	handlers := c.handlers[event]
	if len(handlers) == 0 {
		return nil
	}
	cloned := make([]EventHandler, len(handlers))
	copy(cloned, handlers)
	return cloned
}

func (c *socketConn) enqueue(msg outboundMessage) error {
	select {
	case <-c.closed:
		return ErrConnClosed
	default:
	}
	select {
	case c.send <- msg:
		return nil
	default:
		return ErrBufferFull
	}
}

func (c *socketConn) leaveAllRooms() {
	c.roomsMu.Lock()
	rooms := make([]string, 0, len(c.rooms))
	for name := range c.rooms {
		rooms = append(rooms, name)
	}
	c.rooms = make(map[string]struct{})
	c.roomsMu.Unlock()
	for _, room := range rooms {
		if rm := c.namespace.getRoom(room); rm != nil {
			rm.RemoveByID(c.id)
		}
	}
}

func (c *socketConn) heartbeatLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.pingInterval)
	defer ticker.Stop()

	lastPong := time.Now()
	for {
		select {
		case <-c.closed:
			return
		case <-ticker.C:
			if time.Since(lastPong) >= c.pingTimeout {
				Logger().Warnf("connection heartbeat timeout conn=%s", c.id)
				_ = c.Close()
				return
			}
			if err := c.Emit("ping", nil); err != nil {
				Logger().Warnf("send ping failed conn=%s err=%v", c.id, err)
			}
		case <-c.pongCh:
			lastPong = time.Now()
		}
	}
}

func (c *socketConn) handleBuiltinEvent(frame *Frame) bool {
	if c.pingInterval <= 0 || c.pingTimeout <= 0 {
		return false
	}
	switch frame.Event {
	case "ping":
		if err := c.Emit("pong", nil); err != nil {
			Logger().Warnf("respond pong failed conn=%s err=%v", c.id, err)
		}
		return true
	case "pong":
		select {
		case c.pongCh <- struct{}{}:
		default:
		}
		return true
	default:
		return false
	}
}
