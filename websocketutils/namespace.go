package websocketutils

import (
	"maps"
	"sync"
)

// Namespace 代表具备中间件及房间能力的命名空间。
type Namespace struct {
	name   string
	server *Server

	handlerMu sync.RWMutex
	handlers  map[string][]EventHandler

	middlewares middlewareChain
	hub         *connHub
	rooms       *roomRegistry
}

func newNamespace(server *Server, name string) *Namespace {
	ns := &Namespace{
		name:     name,
		server:   server,
		handlers: make(map[string][]EventHandler),
		hub:      newConnHub(),
	}
	ns.rooms = newRoomRegistry(ns)
	return ns
}

// Name 返回命名空间名称。
func (n *Namespace) Name() string {
	return n.name
}

// On 绑定命名空间事件。
func (n *Namespace) On(event string, handler EventHandler) {
	if event == "" || handler == nil {
		return
	}
	n.handlerMu.Lock()
	n.handlers[event] = append(n.handlers[event], handler)
	n.handlerMu.Unlock()
}

// Use 增加命名空间中间件。
func (n *Namespace) Use(mw Middleware) {
	n.middlewares.Add(mw)
}

// Emit 广播事件到整个命名空间。
func (n *Namespace) Emit(event string, payload any) error {
	frame, err := encodeFrame(event, payload)
	if err != nil {
		return err
	}
	return n.hub.Deliver(frame, n.hub.Snapshot())
}

// To 返回房间广播器。
func (n *Namespace) To(room string) TargetEmitter {
	return (&roomEmitter{
		namespace: n,
	}).To(room)
}

// Room 返回指定房间，没有则创建。
func (n *Namespace) Room(name string) *Room {
	return n.rooms.GetOrCreate(name)
}

func (n *Namespace) runMiddlewares(conn *Conn) error {
	return n.middlewares.Run(conn)
}

func (n *Namespace) attachConnection(conn *Conn) {
	n.hub.Add(conn)
	handlers := n.handlersSnapshot(EventConnection)
	for _, handler := range handlers {
		handler(conn)
	}
}

func (n *Namespace) detachConnection(conn *Conn) {
	n.hub.Remove(conn.id)
}

func (n *Namespace) joinRoom(name string, conn *Conn) error {
	return n.rooms.Join(name, conn)
}

func (n *Namespace) leaveRoom(name string, conn *Conn) {
	n.rooms.Leave(name, conn)
}

func (n *Namespace) handlersSnapshot(event string) []EventHandler {
	n.handlerMu.RLock()
	defer n.handlerMu.RUnlock()
	return append([]EventHandler(nil), n.handlers[event]...)
}

// roomEmitter 实现 TargetEmitter。
type roomEmitter struct {
	namespace *Namespace
	targets   []string
}

// To 追加房间目标。
func (r *roomEmitter) To(room string) TargetEmitter {
	if r == nil || room == "" {
		return r
	}
	r.targets = append(r.targets, room)
	return r
}

// Emit 将事件发送到指定房间。
func (r *roomEmitter) Emit(event string, payload any) error {
	if r == nil || r.namespace == nil {
		return ErrNoSuchRoom
	}
	frame, err := encodeFrame(event, payload)
	if err != nil {
		return err
	}

	roomSet := make(map[string]struct{})
	for _, name := range r.targets {
		if name == "" {
			continue
		}
		roomSet[name] = struct{}{}
	}
	if len(roomSet) == 0 {
		return ErrNoSuchRoom
	}

	rooms := r.namespace.rooms.Select(roomSet)
	if len(rooms) == 0 {
		return ErrNoSuchRoom
	}

	recipients := make(map[string]*Conn)
	for _, room := range rooms {
		room.mu.RLock()
		maps.Copy(recipients, room.members)
		room.mu.RUnlock()
	}

	buffer := make([]*Conn, 0, len(recipients))
	for _, conn := range recipients {
		buffer = append(buffer, conn)
	}
	return r.namespace.hub.Deliver(frame, buffer)
}
