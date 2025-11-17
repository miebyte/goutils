package websocketutils

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Server struct {
	upgrader websocket.Upgrader

	mu         sync.RWMutex
	clients    map[*Client]struct{}
	namespaces map[string]*Namespace

	onConnect func(*Client)
	onClose   func(*Client)

	writeTimeout time.Duration
	readTimeout  time.Duration
	pingInterval time.Duration
}

// NewServer 创建 Server
func NewServer(opts ...Option) *Server {
	s := &Server{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
		clients:      make(map[*Client]struct{}),
		namespaces:   make(map[string]*Namespace),
		writeTimeout: defaultWriteTimeout,
		readTimeout:  defaultReadTimeout,
		pingInterval: 15 * time.Second,
	}
	for _, opt := range opts {
		opt(s)
	}
	s.Namespace(defaultNamespace)
	return s
}

// WithOrigins 设置允许的 Origin
func WithOrigins(origins ...string) Option {
	allowed := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		allowed[o] = struct{}{}
	}
	return func(s *Server) {
		s.upgrader.CheckOrigin = func(r *http.Request) bool {
			if len(allowed) == 0 {
				return true
			}
			if _, ok := allowed[r.Header.Get("Origin")]; ok {
				return true
			}
			return false
		}
	}
}

// WithBufferSize 调整读写缓冲区
func WithBufferSize(read, write int) Option {
	return func(s *Server) {
		s.upgrader.ReadBufferSize = read
		s.upgrader.WriteBufferSize = write
	}
}

// WithHooks 注册连接钩子
func WithHooks(onConnect, onClose func(*Client)) Option {
	return func(s *Server) {
		s.onConnect = onConnect
		s.onClose = onClose
	}
}

// WithTimeout 配置读写超时
func WithTimeout(read, write time.Duration) Option {
	return func(s *Server) {
		if read > 0 {
			s.readTimeout = read
		}
		if write > 0 {
			s.writeTimeout = write
		}
	}
}

// WithPingInterval 配置心跳
func WithPingInterval(interval time.Duration) Option {
	return func(s *Server) {
		if interval > 0 {
			s.pingInterval = interval
		}
	}
}

// Namespace 获取或创建命名空间
func (s *Server) Namespace(name string) *Namespace {
	if name == "" {
		name = defaultNamespace
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ns, ok := s.namespaces[name]; ok {
		return ns
	}
	ns := newNamespace(s, name)
	s.namespaces[name] = ns
	return ns
}

// On 注册默认命名空间事件处理器
func (s *Server) On(event string, handler EventHandler) {
	s.Namespace(defaultNamespace).On(event, handler)
}

// ServeHTTP 处理 websocket 升级
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	client := newClient(s, conn)
	s.registerClient(client)
	if s.onConnect != nil {
		s.onConnect(client)
	}
	go client.writeLoop()
	go client.readLoop()
}

// Emit 向默认命名空间广播事件
func (s *Server) Emit(event string, payload any) error {
	return s.Namespace(defaultNamespace).Emit(event, payload)
}

// EmitExcept 向默认命名空间广播并排除指定客户端
func (s *Server) EmitExcept(event string, payload any, exclude *Client) error {
	return s.Namespace(defaultNamespace).EmitExcept(event, payload, exclude)
}

// EmitToRoom 向默认命名空间的房间广播
func (s *Server) EmitToRoom(room, event string, payload any) error {
	return s.Namespace(defaultNamespace).EmitToRoom(room, event, payload)
}

// EmitToRoomExcept 向默认命名空间房间广播并排除指定客户端
func (s *Server) EmitToRoomExcept(room, event string, payload any, exclude *Client) error {
	return s.Namespace(defaultNamespace).EmitToRoomExcept(room, event, payload, exclude)
}

func (s *Server) registerClient(c *Client) {
	s.mu.Lock()
	s.clients[c] = struct{}{}
	s.mu.Unlock()
	s.joinNamespace(c, defaultNamespace)
}

func (s *Server) unregisterClient(c *Client) {
	s.mu.Lock()
	delete(s.clients, c)
	for _, ns := range s.namespaces {
		ns.removeClient(c)
	}
	s.mu.Unlock()
	if s.onClose != nil {
		s.onClose(c)
	}
}

func (s *Server) joinNamespace(c *Client, namespace string) {
	ns := s.Namespace(namespace)
	ns.addClient(c)
	c.addNamespace(namespace)
}

func (s *Server) leaveNamespace(c *Client, namespace string) {
	s.mu.RLock()
	ns, ok := s.namespaces[namespace]
	s.mu.RUnlock()
	if !ok {
		return
	}
	ns.removeClient(c)
	c.removeNamespace(namespace)
}

func (s *Server) joinRoom(namespace string, c *Client, room string) {
	ns := s.Namespace(namespace)
	ns.addClient(c)
	ns.joinRoom(c, room)
	c.addRoom(namespace, room)
}

func (s *Server) leaveRoom(namespace string, c *Client, room string) {
	s.mu.RLock()
	ns, ok := s.namespaces[namespace]
	s.mu.RUnlock()
	if !ok {
		return
	}
	ns.leaveRoom(c, room)
	c.removeRoom(namespace, room)
}

func (s *Server) dispatchEvent(namespace string, c *Client, msg *WireMessage) {
	if !c.inNamespace(namespace) {
		errResp := WireMessage{Type: MessageTypeError, Namespace: namespace, Error: "namespace not joined"}
		b, _ := json.Marshal(errResp)
		c.enqueue(b)
		return
	}
	ns := s.Namespace(namespace)
	ns.dispatchEvent(c, msg)
}
