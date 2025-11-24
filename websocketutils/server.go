package websocketutils

import (
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var _ ServerAPI = (*Server)(nil)

const (
	defaultNamespaceName = "default"
	defaultSendQueueSize = 64
)

// Option 定义 Server 可选项。
type Option func(*Server)

// Server 提供命名空间与房间管理。
type Server struct {
	mu              sync.RWMutex
	namespaces      map[string]*namespace
	upgrader        websocket.Upgrader
	namespacePrefix string
	sendQueueSize   int
	allowRequest    func(*http.Request) (*http.Request, error)
	pingInterval    time.Duration
	pingTimeout     time.Duration
}

// NewServer 创建一个 Server。
func NewServer(opts ...Option) ServerAPI {
	s := &Server{
		namespaces: make(map[string]*namespace),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(*http.Request) bool {
				return true
			},
		},
		sendQueueSize:   defaultSendQueueSize,
		namespacePrefix: "/",
		pingInterval:    0,
		pingTimeout:     0,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

// WithCheckOrigin 自定义升级检查。
func WithCheckOrigin(fn func(*http.Request) bool) Option {
	return func(s *Server) {
		if fn != nil {
			s.upgrader.CheckOrigin = fn
		}
	}
}

// WithReadBufferSize 设置升级器读缓冲大小。
func WithReadBufferSize(size int) Option {
	return func(s *Server) {
		if size > 0 {
			s.upgrader.ReadBufferSize = size
		}
	}
}

// WithWriteBufferSize 设置升级器写缓冲大小。
func WithWriteBufferSize(size int) Option {
	return func(s *Server) {
		if size > 0 {
			s.upgrader.WriteBufferSize = size
		}
	}
}

// WithSendQueueSize 设置连接发送缓冲大小。
func WithSendQueueSize(size int) Option {
	return func(s *Server) {
		if size > 0 {
			s.sendQueueSize = size
		}
	}
}

// WithNamespacePrefix 设置命名空间路径前缀。
func WithNamespacePrefix(prefix string) Option {
	return func(s *Server) {
		s.namespacePrefix = sanitizeNamespacePrefix(prefix)
	}
}

// WithAllowRequestFunc 允许在校验后替换请求。
func WithAllowRequestFunc(fn AllowRequestFunc) Option {
	if fn == nil {
		return nil
	}
	return func(s *Server) {
		s.allowRequest = fn
	}
}

// WithHeartbeat 启用心跳机制。
func WithHeartbeat(interval, timeout time.Duration) Option {
	return func(s *Server) {
		if interval <= 0 || timeout <= 0 {
			s.pingInterval = 0
			s.pingTimeout = 0
			return
		}
		s.pingInterval = interval
		s.pingTimeout = timeout
	}
}

func (s *Server) normalizeNamespace(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "/")
	if name == "" {
		return defaultNamespaceName
	}
	chunks := strings.Split(name, "/")
	if len(chunks) > 0 && chunks[0] != "" {
		return chunks[0]
	}
	return defaultNamespaceName
}

func (s *Server) getNamespace(name string) *namespace {
	norm := s.normalizeNamespace(name)
	s.mu.RLock()
	ns, ok := s.namespaces[norm]
	s.mu.RUnlock()
	if ok {
		return ns
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ns, ok = s.namespaces[norm]; ok {
		return ns
	}
	ns = newNamespace(norm)
	s.namespaces[norm] = ns
	return ns
}

// Of 返回命名空间。
func (s *Server) Of(name string) NamespaceAPI {
	return s.getNamespace(name)
}

// Broadcast 广播到所有命名空间。
func (s *Server) Broadcast(event string, payload any) {
	s.mu.RLock()
	namespaces := make([]*namespace, 0, len(s.namespaces))
	for _, ns := range s.namespaces {
		namespaces = append(namespaces, ns)
	}
	s.mu.RUnlock()
	for _, ns := range namespaces {
		if err := ns.Emit(event, payload); err != nil {
			Logger().Errorf("namespace broadcast failed namespace=%s event=%s err=%v", ns.Name(), event, err)
		}
	}
}

// ServeHTTP 实现 http.Handler。
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req := r
	if s.allowRequest != nil {
		newReq, err := s.allowRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if newReq != nil {
			req = newReq
		}
	}

	nsName := s.namespaceFromPath(req.URL.Path)
	ns := s.getNamespace(nsName)
	conn, err := s.upgrader.Upgrade(w, req, nil)
	if err != nil {
		Logger().Errorf("websocket upgrade failed err=%v", err)
		return
	}
	transport := newWSTransport(conn)
	socket := newSocketConn(
		ns,
		transport,
		req,
		s.sendQueueSize,
		s.pingInterval,
		s.pingTimeout,
	)

	ns.addConn(socket)
	if err := socket.Join(socket.ID()); err != nil {
		Logger().Warnf("join self room failed conn=%s err=%v", socket.ID(), err)
	}
	ns.dispatch(EventConnection, socket, nil)
	socket.run()
}

func sanitizeNamespacePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "/"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	cleaned := path.Clean(prefix)
	if cleaned == "." || cleaned == "" {
		return "/"
	}
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	return cleaned
}

func (s *Server) namespaceFromPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return defaultNamespaceName
	}
	prefix := s.namespacePrefix
	if prefix == "" {
		prefix = "/"
	}
	if prefix != "/" {
		if !strings.HasPrefix(p, prefix) {
			return defaultNamespaceName
		}
		if len(p) > len(prefix) && p[len(prefix)] != '/' {
			return defaultNamespaceName
		}
		p = p[len(prefix):]
	}
	p = strings.TrimPrefix(p, "/")
	return s.normalizeNamespace(p)
}
