package websocketutils

import "sync"

// room 表示命名空间房间。
type room struct {
	name    string
	mu      sync.RWMutex
	members map[string]Conn
}

func newRoom(name string) *room {
	return &room{
		name:    name,
		members: make(map[string]Conn),
	}
}

func (r *room) Name() string {
	return r.name
}

func (r *room) Broadcast(event string, data any) {
	members := r.snapshot()
	for _, conn := range members {
		if err := conn.Emit(event, data); err != nil {
			Logger().Errorf("room broadcast failed room=%s conn=%s event=%s err=%v", r.name, conn.ID(), event, err)
		}
	}
}

func (r *room) Add(conn Conn) {
	if conn == nil {
		return
	}
	r.mu.Lock()
	r.members[conn.ID()] = conn
	r.mu.Unlock()
}

func (r *room) Remove(conn Conn) {
	if conn == nil {
		return
	}
	r.RemoveByID(conn.ID())
}

func (r *room) RemoveByID(id string) {
	if id == "" {
		return
	}
	r.mu.Lock()
	delete(r.members, id)
	r.mu.Unlock()
}

func (r *room) Members() []Conn {
	return r.snapshot()
}

func (r *room) snapshot() []Conn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.members) == 0 {
		return nil
	}
	list := make([]Conn, 0, len(r.members))
	for _, conn := range r.members {
		list = append(list, conn)
	}
	return list
}

var _ RoomAPI = (*room)(nil)
