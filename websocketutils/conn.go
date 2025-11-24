package websocketutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/miebyte/goutils/logging"
	cmap "github.com/orcaman/concurrent-map/v2"
)

const pingWriteWait = 5 * time.Second

// socket 实现了 Conn 接口，封装了 Transport 和业务逻辑
type socket struct {
	id        string
	namespace *Namespace
	transport Transport
	req       *http.Request

	sendCh      chan []byte
	middlewares middlewareChain
	handlers    cmap.ConcurrentMap[string, []EventHandler]
	rooms       cmap.ConcurrentMap[string, struct{}]
	keys        cmap.ConcurrentMap[string, any]

	closeOnce sync.Once
	closed    chan struct{}

	ctx    context.Context
	cancel context.CancelFunc

	pingInterval time.Duration
	pongTimeout  time.Duration
}

func newSocket(ctx context.Context, ns *Namespace, transport Transport, req *http.Request) *socket {
	id := nextConnID()
	ctx, cancel := context.WithCancel(ctx)
	ctx = logging.With(ctx, "ConnID", id)
	pingInterval, pongTimeout := ns.server.heartbeatConfig()

	s := &socket{
		id:           id,
		namespace:    ns,
		transport:    transport,
		req:          req,
		sendCh:       make(chan []byte, 16),
		handlers:     cmap.New[[]EventHandler](),
		rooms:        cmap.New[struct{}](),
		keys:         cmap.New[any](),
		closed:       make(chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
		pingInterval: pingInterval,
		pongTimeout:  pongTimeout,
	}

	s.setupHeartbeat()
	go s.readLoop()
	go s.writeLoop()
	return s
}

// ID 返回连接 ID。
func (s *socket) ID() string {
	return s.id
}

// Namespace 返回命名空间。
func (s *socket) Namespace() NamespaceAPI {
	return s.namespace
}

// Request 返回握手请求。
func (s *socket) Request() *http.Request {
	return s.req
}

// Context 返回连接上下文。
func (s *socket) Context() context.Context {
	return s.ctx
}

func (s *socket) SetContext(ctx context.Context) {
	s.ctx = ctx
}

// Set 存储键值对。
func (s *socket) Set(key string, value any) {
	s.keys.Set(key, value)
}

// Get 获取键值对。
func (s *socket) Get(key string) (any, bool) {
	return s.keys.Get(key)
}

// On 绑定事件处理函数。
func (s *socket) On(event string, handler EventHandler) {
	if event == "" || handler == nil {
		return
	}
	s.handlers.Upsert(event, []EventHandler{handler}, func(exist bool, oldVal []EventHandler, newVal []EventHandler) []EventHandler {
		return append(oldVal, newVal...)
	})
}

// Use 增加中间件。
func (s *socket) Use(mw Middleware) {
	s.middlewares.Add(mw)
}

// To 返回房间广播器。
func (s *socket) To(rooms ...string) TargetEmitter {
	return (&roomEmitter{
		namespace: s.namespace,
		except:    s,
	}).To(rooms...)
}

// Emit 发送事件。
func (s *socket) Emit(event string, payload any) error {
	frame, err := encodeFrame(event, payload)
	if err != nil {
		return err
	}
	return s.SendFrame(frame)
}

// Join 将连接加入房间。
func (s *socket) Join(room string) error {
	if room == "" {
		return ErrNoSuchRoom
	}
	return s.namespace.joinRoom(room, s)
}

// Leave 将连接移出房间。
func (s *socket) Leave(room string) {
	if room == "" {
		return
	}
	s.namespace.leaveRoom(room, s)
}

// Rooms 返回当前连接所在的房间列表。
func (s *socket) Rooms() []string {
	return s.rooms.Keys()
}

// Close 关闭连接。
func (s *socket) Close() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.closed)
		s.cancel()
		s.namespace.detachConnection(s)

		rooms := s.Rooms()
		for _, room := range rooms {
			s.namespace.leaveRoom(room, s)
		}

		err = s.transport.Close()
	})
	return err
}

func (s *socket) readLoop() {
	defer s.Close()
	for {
		frame, err := s.transport.Read()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				s.namespace.onError(s, err)
			} else {
				websocketLogger.Infoc(s.ctx, "connection closed")
			}
			return
		}

		if frame.Event == "" {
			websocketLogger.Warnc(s.ctx, "read empty event")
			continue
		}

		websocketLogger.Debugc(s.ctx, "read event=%s", logging.JsonifyNoIndent(frame))
		s.dispatch(frame)
	}
}

func (s *socket) writeLoop() {
	var (
		ticker *time.Ticker
		pingC  <-chan time.Time
	)
	if s.pingInterval > 0 {
		ticker = time.NewTicker(s.pingInterval)
		pingC = ticker.C
		defer ticker.Stop()
	}

	defer s.Close()

	for {
		select {
		case <-s.closed:
			return
		case payload, ok := <-s.sendCh:
			if !ok {
				return
			}
			if err := s.transport.Write(websocket.TextMessage, payload); err != nil {
				s.namespace.onError(s, err)
				return
			}
		case <-pingC:
			if err := s.sendPing(); err != nil {
				s.namespace.onError(s, err)
				return
			}
		}
	}
}

func (s *socket) dispatch(frame *Frame) {
	handlers, _ := s.handlers.Get(frame.Event)

	if len(handlers) == 0 {
		return
	}

	wrapper := &socketWrapper{
		Conn: s,
		ctx:  context.WithValue(s.Context(), dataKey, frame.Data),
	}

	if err := s.middlewares.Run(wrapper); err != nil {
		s.namespace.onError(s, err)
		return
	}

	for _, handler := range handlers {
		go func(h EventHandler) {
			defer func() {
				if r := recover(); r != nil {
					s.namespace.onError(s, fmt.Errorf("handler panic: %v", r))
				}
			}()
			h(wrapper)
		}(handler)
	}
}

type dataKeyType struct{}

var dataKey dataKeyType

// GetFrameData 获取当前事件帧数据。
func GetFrameData(s Conn) json.RawMessage {
	if v := s.Context().Value(dataKey); v != nil {
		if data, ok := v.(json.RawMessage); ok {
			return data
		}
	}
	return nil
}

type socketWrapper struct {
	Conn
	ctx context.Context
}

func (w *socketWrapper) Context() context.Context {
	return w.ctx
}

func (w *socketWrapper) SetContext(ctx context.Context) {
	w.ctx = ctx
}

// SendFrame 实现 FrameSender 接口
func (s *socket) SendFrame(data []byte) error {
	select {
	case <-s.closed:
		return ErrConnClosed
	case s.sendCh <- data:
		return nil
	default:
		return errors.New("websocketutils: connection buffer full")
	}
}

func (s *socket) addRoom(name string) {
	s.rooms.Set(name, struct{}{})
}

func (s *socket) removeRoom(name string) {
	s.rooms.Remove(name)
}

func (s *socket) setupHeartbeat() {
	if s.pongTimeout <= 0 {
		return
	}
	if err := s.refreshReadDeadline(); err != nil {
		websocketLogger.Warnc(s.ctx, "set read deadline error: %v", err)
	}
	s.transport.SetPongHandler(func(string) error {
		if err := s.refreshReadDeadline(); err != nil {
			websocketLogger.Warnc(s.ctx, "refresh read deadline error: %v", err)
			return err
		}
		return nil
	})
}

func (s *socket) refreshReadDeadline() error {
	if s.pongTimeout <= 0 {
		return nil
	}
	return s.transport.SetReadDeadline(time.Now().Add(s.pongTimeout))
}

func (s *socket) sendPing() error {
	if s.pingInterval <= 0 {
		return nil
	}
	websocketLogger.Debugc(s.ctx, "sending ping")
	return s.transport.WriteControl(websocket.PingMessage, nil, time.Now().Add(pingWriteWait))
}

func nextConnID() string {
	return uuid.NewString()
}
