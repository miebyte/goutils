package websocketutils

import (
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// Server 是具备命名空间能力的 WebSocket 服务器。
type Server struct {
	upgrader    websocket.Upgrader
	mu          sync.RWMutex
	namespaces  map[string]*Namespace
	middlewares []Middleware
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
		namespaces: make(map[string]*Namespace),
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

// ServeHTTP 实现 http.Handler。
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	namespace := s.Of(r.URL.Path)
	if namespace == nil {
		http.Error(w, "namespace not found", http.StatusNotFound)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	socket := newConn(namespace, conn)
	if err := s.runMiddlewares(socket); err != nil {
		_ = socket.Close()
		return
	}
	if err := namespace.runMiddlewares(socket); err != nil {
		_ = socket.Close()
		return
	}
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

// Of 返回或创建命名空间。
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
	}
	return ns
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
