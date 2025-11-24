package websocketutils

import (
	"encoding/json"
	"sync"
)

// namespace 管理命名空间连接。
type namespace struct {
	name     string
	mu       sync.RWMutex
	conns    map[string]Conn
	rooms    map[string]*room
	handlers map[string][]EventHandler
}

func newNamespace(name string) *namespace {
	return &namespace{
		name:     name,
		conns:    make(map[string]Conn),
		rooms:    make(map[string]*room),
		handlers: make(map[string][]EventHandler),
	}
}

func (n *namespace) Name() string {
	return n.name
}

func (n *namespace) Emit(event string, payload any) error {
	targets := n.cloneConns(nil)
	for _, conn := range targets {
		if err := conn.Emit(event, payload); err != nil {
			Logger().Errorf("namespace emit failed namespace=%s conn=%s event=%s err=%v", n.name, conn.ID(), event, err)
		}
	}
	return nil
}

func (n *namespace) EmitExcept(event string, payload any, except Conn) error {
	targets := n.cloneConns(except)
	for _, conn := range targets {
		if err := conn.Emit(event, payload); err != nil {
			Logger().Errorf("namespace emit except failed namespace=%s conn=%s event=%s err=%v", n.name, conn.ID(), event, err)
		}
	}
	return nil
}

func (n *namespace) On(event string, handler EventHandler) {
	if event == "" || handler == nil {
		return
	}
	n.mu.Lock()
	n.handlers[event] = append(n.handlers[event], handler)
	n.mu.Unlock()
}

func (n *namespace) Room(name string) RoomAPI {
	if name == "" {
		return nil
	}
	if room := n.getRoom(name); room != nil {
		return room
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if room, ok := n.rooms[name]; ok {
		return room
	}
	room := newRoom(name)
	n.rooms[name] = room
	return room
}

func (n *namespace) addConn(conn Conn) {
	n.mu.Lock()
	n.conns[conn.ID()] = conn
	n.mu.Unlock()
}

func (n *namespace) removeConn(id string) {
	n.mu.Lock()
	delete(n.conns, id)
	for _, rm := range n.rooms {
		rm.RemoveByID(id)
	}
	n.mu.Unlock()
}

// dispatch 派发事件到命名空间。
func (n *namespace) dispatch(event string, conn Conn, data json.RawMessage) {
	n.mu.RLock()
	handlers := append([]EventHandler(nil), n.handlers[event]...)
	n.mu.RUnlock()
	if len(handlers) == 0 {
		return
	}

	ctx := newEventContext(conn.Context(), conn, n, event, data)
	for _, h := range handlers {
		callHandlerSafely(h, ctx)
	}
}

func (n *namespace) cloneConns(except Conn) []Conn {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if len(n.conns) == 0 {
		return nil
	}
	targets := make([]Conn, 0, len(n.conns))
	for id, conn := range n.conns {
		if except != nil && id == except.ID() {
			continue
		}
		targets = append(targets, conn)
	}
	return targets
}

func (n *namespace) getRoom(name string) *room {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.rooms[name]
}

var _ NamespaceAPI = (*namespace)(nil)
