package websocketutils

import (
	cmap "github.com/orcaman/concurrent-map/v2"
)

var _ NamespaceAPI = (*Namespace)(nil)

type Namespace struct {
	name         string
	server       *Server
	rooms        cmap.ConcurrentMap[string, *Room]
	connections  cmap.ConcurrentMap[string, Conn]
	middlewares  middlewareChain
	handlers     cmap.ConcurrentMap[string, []EventHandler]
	errorHandler ErrorHandler
}

func newNamespace(server *Server, name string) *Namespace {
	return &Namespace{
		name:        name,
		server:      server,
		rooms:       cmap.New[*Room](),
		connections: cmap.New[Conn](),
		handlers:    cmap.New[[]EventHandler](),
	}
}

func (n *Namespace) Name() string {
	return n.name
}

// On 绑定事件处理函数
// 目前主要用于绑定 connection 事件
func (n *Namespace) On(event string, handler EventHandler) {
	if event == "" || handler == nil {
		return
	}
	n.handlers.Upsert(event, []EventHandler{handler}, func(exist bool, oldVal []EventHandler, newVal []EventHandler) []EventHandler {
		return append(oldVal, newVal...)
	})
}

// OnError 绑定错误处理函数
func (n *Namespace) OnError(handler ErrorHandler) {
	n.errorHandler = handler
}

func (n *Namespace) onError(conn Conn, err error) {
	if n.errorHandler != nil {
		n.errorHandler(conn, err)
		return
	}
	websocketLogger.Errorf("websocket error: %v", err)
}

// Use 增加中间件
func (n *Namespace) Use(middleware Middleware) {
	n.middlewares.Add(middleware)
}

// Room 返回一个房间，如果不存在则创建
func (n *Namespace) Room(name string) RoomAPI {
	if room, ok := n.rooms.Get(name); ok {
		return room
	}

	newR := newRoom(n, name)
	if n.rooms.SetIfAbsent(name, newR) {
		return newR
	}
	// already exists
	r, _ := n.rooms.Get(name)
	return r
}

// To 返回房间广播器
func (n *Namespace) To(rooms ...string) TargetEmitter {
	return (&roomEmitter{
		namespace: n,
	}).To(rooms...)
}

// Emit 广播事件到整个命名空间
func (n *Namespace) Emit(event string, data any) error {
	// 广播给所有连接
	frame, err := encodeFrame(event, data)
	if err != nil {
		return err
	}
	for _, conn := range n.connections.Items() {
		err = conn.SendFrame(frame)
		if err != nil {
			websocketLogger.Errorf("send frame error: %v", err)
		}
	}
	return nil
}

func (n *Namespace) runMiddlewares(conn Conn) error {
	return n.middlewares.Run(conn)
}

func (n *Namespace) attachConnection(conn Conn) {
	n.connections.Set(conn.ID(), conn)

	// 触发 connection 事件
	handlers, ok := n.handlers.Get(EventConnection)
	if ok {
		for _, h := range handlers {
			go h(conn)
		}
	}
}

func (n *Namespace) detachConnection(conn Conn) {
	n.connections.Remove(conn.ID())

	// 触发 disconnect 事件
	handlers, ok := n.handlers.Get(EventDisconnect)
	if ok {
		for _, h := range handlers {
			go h(conn)
		}
	}
}

func (n *Namespace) joinRoom(roomName string, s *socket) error {
	r := n.Room(roomName)
	r.Add(s)
	s.addRoom(roomName)
	return nil
}

func (n *Namespace) leaveRoom(roomName string, s *socket) {
	r, ok := n.rooms.Get(roomName)
	if ok {
		r.Remove(s)
	}
	s.removeRoom(roomName)
}

// roomEmitter implementation
type roomEmitter struct {
	namespace *Namespace
	rooms     []string
	except    Conn
}

func (e *roomEmitter) To(rooms ...string) TargetEmitter {
	e.rooms = append(e.rooms, rooms...)
	return e
}

func (e *roomEmitter) EmitExcept(event string, data any, except Conn) error {
	e.except = except
	return e.Emit(event, data)
}

func (e *roomEmitter) Emit(event string, data any) error {
	frame, err := encodeFrame(event, data)
	if err != nil {
		return err
	}

	// Collect all unique connections from target rooms
	targetConns := make(map[string]Conn)

	if len(e.rooms) == 0 {
		// broadcast to all connections
		for _, conn := range e.namespace.connections.Items() {
			if e.except != nil && conn.ID() == e.except.ID() {
				continue
			}
			err = conn.SendFrame(frame)
			if err != nil {
				// Use namespace error handler
				e.namespace.onError(conn, err)
			}
		}
		return nil
	}

	for _, roomName := range e.rooms {
		room := e.namespace.Room(roomName)
		for _, m := range room.Members() {
			targetConns[m.ID()] = m
		}
	}

	for id, conn := range targetConns {
		if e.except != nil && id == e.except.ID() {
			continue
		}
		err = conn.SendFrame(frame)
		if err != nil {
			// Use namespace error handler
			e.namespace.onError(conn, err)
		}
	}
	return nil
}
