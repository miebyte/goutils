package websocketutils

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	cmap "github.com/orcaman/concurrent-map/v2"
)

const (
	defaultPingInterval = 25 * time.Second
	defaultPongTimeout  = 60 * time.Second
)

type Server struct {
	upgrader     websocket.Upgrader
	namespaces   cmap.ConcurrentMap[string, *Namespace]
	pingInterval time.Duration
	pongTimeout  time.Duration
	handshake    HandshakeFunc
	pathPrefix   string
}

type ServerOption func(*Server)

type HandshakeFunc func(r *http.Request) (context.Context, error)

func NewServer(opts ...ServerOption) *Server {
	srv := &Server{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		namespaces:   cmap.New[*Namespace](),
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

// WithHandshake 配置握手校验函数。
func WithHandshake(fn HandshakeFunc) ServerOption {
	return func(s *Server) {
		s.SetHandshake(fn)
	}
}

// WithPrefix 配置监听路径前缀。
func WithPrefix(prefix string) ServerOption {
	return func(s *Server) {
		s.pathPrefix = normalizePrefix(prefix)
	}
}

func (s *Server) Broadcast(event string, data any) {
	for _, ns := range s.namespaces.Items() {
		ns.Emit(event, data)
	}
}

func (s *Server) Use(middleware Middleware) {
	for _, ns := range s.namespaces.Items() {
		ns.Use(middleware)
	}
}

// ServeHTTP 实现 http.Handler。
// Server 会根据请求的 URL 解析出一个命名空间并自动加入
// 每个 socket 都会自动加入一个由其自己的 id 标识的房间。
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, err := s.checkHandshake(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		websocketLogger.Warnf("websocket request rejected: path=%s err=%v", r.URL.Path, err)
		return
	}

	if ctx != nil {
		r = r.WithContext(ctx)
	}

	nsPath := s.namespacePath(r.URL.Path)
	namespace := s.getNamespace(nsPath)
	if namespace == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		websocketLogger.Warnc(ctx, "websocket namespace not found: path=%s ns=%s", r.URL.Path, nsPath)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	transport := &WebSocketTransport{conn: conn}
	socket := newSocket(r.Context(), namespace, transport, r)
	if err := namespace.runMiddlewares(socket); err != nil {
		_ = socket.Close()
		return
	}

	// 每个 socket 都会自动加入一个由其自己的 id 标识的房间。
	socket.Join(socket.ID())
	// connection 事件处理
	namespace.attachConnection(socket)
}

// Of 返回一个命名空间
// 如果命名空间不存在，则创建一个。
func (s *Server) Of(name string) *Namespace {
	normalized := normalizeNamespace(name)
	ns, _ := s.namespaces.Get(normalized)
	if ns != nil {
		return ns
	}

	ns = newNamespace(s, normalized)
	s.namespaces.Set(normalized, ns)
	websocketLogger.Infof("created namespace: %s", normalized)

	return ns
}

func (s *Server) heartbeatConfig() (time.Duration, time.Duration) {
	return s.pingInterval, s.pongTimeout
}

func (s *Server) SetHandshake(fn HandshakeFunc) {
	s.handshake = fn
}

func (s *Server) checkHandshake(r *http.Request) (context.Context, error) {
	if s.handshake == nil {
		return nil, nil
	}
	return s.handshake(r)
}

func (s *Server) getNamespace(name string) *Namespace {
	normalized := normalizeNamespace(name)
	ns, _ := s.namespaces.Get(normalized)
	return ns
}

func (s *Server) namespacePath(requestPath string) string {
	if requestPath == "" {
		return "/"
	}
	prefix := s.pathPrefix
	if prefix == "" {
		return requestPath
	}
	if !strings.HasPrefix(requestPath, prefix) {
		return requestPath
	}
	if len(requestPath) > len(prefix) && requestPath[len(prefix)] != '/' {
		return requestPath
	}
	trimmed := strings.TrimPrefix(requestPath, prefix)
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return trimmed
}
