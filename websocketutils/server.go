package websocketutils

import (
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/miebyte/goutils/logging"
)

const (
	defaultPingInterval = 25 * time.Second
	defaultPongTimeout  = 60 * time.Second
)

// Server 是具备命名空间能力的 WebSocket 服务器。
type Server struct {
	upgrader    websocket.Upgrader
	mu          sync.RWMutex
	namespaces  map[string]*Namespace
	middlewares []Middleware

	pingInterval time.Duration
	pongTimeout  time.Duration
}

// ServerOption 用于自定义 Server。
type ServerOption func(*Server)

// NewServer 创建一个新的 Server。
func NewServer(opts ...ServerOption) *Server {
	srv := &Server{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		namespaces:   make(map[string]*Namespace),
		pingInterval: defaultPingInterval,
		pongTimeout:  defaultPongTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(srv)
		}
	}
	srv.Of("/")
	return srv
}

// WithUpgrader 替换默认 upgrader。
func WithUpgrader(upgrader websocket.Upgrader) ServerOption {
	return func(s *Server) {
		s.upgrader = upgrader
	}
}

// WithHeartbeat 配置服务端心跳与超时。
func WithHeartbeat(pingInterval, pongTimeout time.Duration) ServerOption {
	return func(s *Server) {
		if pingInterval <= 0 || pongTimeout <= 0 {
			s.pingInterval = 0
			s.pongTimeout = 0
			return
		}
		if pongTimeout <= pingInterval {
			pongTimeout = pingInterval + time.Second
		}
		s.pingInterval = pingInterval
		s.pongTimeout = pongTimeout
	}
}

// ServeHTTP 实现 http.Handler。
// Server 会根据请求的 URL 解析出一个命名空间并自动加入
// 每个 socket 都会自动加入一个由其自己的 id 标识的房间。
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	namespace := s.Of(r.URL.Path)
	socket := newConn(namespace, conn)
	if err := s.runMiddlewares(socket); err != nil {
		_ = socket.Close()
		return
	}
	if err := namespace.runMiddlewares(socket); err != nil {
		_ = socket.Close()
		return
	}

	// 每个 socket 都会自动加入一个由其自己的 id 标识的房间。
	namespace.joinRoom(socket.ID(), socket)
	// connection 事件处理
	namespace.attachConnection(socket)
}

// On 代理默认命名空间的事件绑定。
func (s *Server) On(event string, handler EventHandler) {
	s.Of("/").On(event, handler)
}

// Use 注册全局中间件。
func (s *Server) Use(mw Middleware) {
	if mw == nil {
		return
	}
	s.mu.Lock()
	s.middlewares = append(s.middlewares, mw)
	s.mu.Unlock()
}

// Emit 代理默认命名空间的广播。
func (s *Server) Emit(event string, payload any) error {
	return s.Of("/").Emit(event, payload)
}

// To 代理默认命名空间的房间广播器。
func (s *Server) To(room string) TargetEmitter {
	return s.Of("/").To(room)
}

// Of 返回一个命名空间
// 如果命名空间不存在，则创建一个。
func (s *Server) Of(name string) *Namespace {
	normalized := normalizeNamespace(name)
	s.mu.RLock()
	ns := s.namespaces[normalized]
	s.mu.RUnlock()
	if ns != nil {
		return ns
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ns = s.namespaces[normalized]
	if ns == nil {
		ns = newNamespace(s, normalized)
		s.namespaces[normalized] = ns
		logging.Infof("created namespace: %s", normalized)
	}
	return ns
}

func (s *Server) heartbeatConfig() (time.Duration, time.Duration) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pingInterval, s.pongTimeout
}

func (s *Server) runMiddlewares(conn *Conn) error {
	s.mu.RLock()
	mws := append([]Middleware(nil), s.middlewares...)
	s.mu.RUnlock()
	for _, mw := range mws {
		if err := mw(conn); err != nil {
			return err
		}
	}
	return nil
}

func normalizeNamespace(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." {
		return "/"
	}
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	return cleaned
}
