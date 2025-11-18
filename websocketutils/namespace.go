package websocketutils

import (
	"encoding/json"
	"strings"
	"sync"
)

// Namespace 代表具备中间件及房间能力的命名空间。
type Namespace struct {
	name   string
	server *Server

	mu          sync.RWMutex
	rooms       map[string]*Room
	connections map[string]*Conn
	middlewares []Middleware
	handlers    map[string][]EventHandler
}

func newNamespace(server *Server, name string) *Namespace {
	return &Namespace{
		name:        name,
		server:      server,
		rooms:       make(map[string]*Room),
		connections: make(map[string]*Conn),
		handlers:    make(map[string][]EventHandler),
	}
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
	n.mu.Lock()
	n.handlers[event] = append(n.handlers[event], handler)
	n.mu.Unlock()
}

// Use 增加命名空间中间件。
func (n *Namespace) Use(mw Middleware) {
	if mw == nil {
		return
	}
	n.mu.Lock()
	n.middlewares = append(n.middlewares, mw)
	n.mu.Unlock()
}

// Emit 广播事件到整个命名空间。
func (n *Namespace) Emit(event string, payload any) error {
	frame, err := encodeFrame(event, payload)
	if err != nil {
		return err
	}

	n.mu.RLock()
	receivers := make([]*Conn, 0, len(n.connections))
	for _, conn := range n.connections {
		receivers = append(receivers, conn)
	}
	n.mu.RUnlock()

	var firstErr error
	for _, conn := range receivers {
		if err := conn.sendFrame(frame); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Room 返回指定房间，没有则创建。
func (n *Namespace) Room(name string) *Room {
	if name == "" {
		return nil
	}
	n.mu.Lock()
	room := n.rooms[name]
	if room == nil {
		room = newRoom(n, name)
		n.rooms[name] = room
	}
	n.mu.Unlock()
	return room
}

func (n *Namespace) runMiddlewares(conn *Conn) error {
	n.mu.RLock()
	mws := append([]Middleware(nil), n.middlewares...)
	n.mu.RUnlock()
	for _, mw := range mws {
		if err := mw(conn); err != nil {
			return err
		}
	}
	return nil
}

func (n *Namespace) attachConnection(conn *Conn) {
	n.bindDefaultSocketEvents(conn)

	n.mu.Lock()
	n.connections[conn.id] = conn
	handlers := append([]EventHandler(nil), n.handlers[EventConnection]...)
	n.mu.Unlock()
	for _, handler := range handlers {
		handler(conn)
	}
}

func (n *Namespace) bindDefaultSocketEvents(conn *Conn) {
	conn.On(EventJoinRoom, func(_ Socket, data json.RawMessage) {
		if room := parseRoomName(data); room != "" {
			_ = n.joinRoom(room, conn)
		}
	})

	conn.On(EventLeaveRoom, func(_ Socket, data json.RawMessage) {
		if room := parseRoomName(data); room != "" {
			n.leaveRoom(room, conn)
		}
	})
}

func (n *Namespace) detachConnection(conn *Conn) {
	n.mu.Lock()
	delete(n.connections, conn.id)
	n.mu.Unlock()
}

func (n *Namespace) joinRoom(name string, conn *Conn) error {
	room := n.Room(name)
	if room == nil {
		return ErrNoSuchRoom
	}
	room.add(conn)
	conn.addRoom(name)
	return nil
}

func (n *Namespace) leaveRoom(name string, conn *Conn) {
	n.mu.RLock()
	room := n.rooms[name]
	n.mu.RUnlock()
	if room == nil {
		return
	}
	empty := room.remove(conn.id)
	conn.removeRoom(name)
	if empty {
		n.mu.Lock()
		if current, ok := n.rooms[name]; ok && current == room {
			delete(n.rooms, name)
		}
		n.mu.Unlock()
	}
}

func parseRoomName(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var payload struct {
		Room string `json:"room"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Room)
}
