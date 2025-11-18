package websocketutils

import "sync"

// connHub 管理命名空间内的所有连接。
type connHub struct {
	mu    sync.RWMutex
	conns map[string]*Conn
}

func newConnHub() *connHub {
	return &connHub{
		conns: make(map[string]*Conn),
	}
}

func (h *connHub) Add(conn *Conn) {
	if conn == nil {
		return
	}
	h.mu.Lock()
	h.conns[conn.id] = conn
	h.mu.Unlock()
}

func (h *connHub) Remove(connID string) {
	if connID == "" {
		return
	}
	h.mu.Lock()
	delete(h.conns, connID)
	h.mu.Unlock()
}

func (h *connHub) Snapshot() []*Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	receivers := make([]*Conn, 0, len(h.conns))
	for _, conn := range h.conns {
		receivers = append(receivers, conn)
	}
	return receivers
}

func (h *connHub) Deliver(frame []byte, receivers []*Conn) error {
	if len(receivers) == 0 {
		return nil
	}
	var firstErr error
	for _, conn := range receivers {
		if conn == nil {
			continue
		}
		if err := conn.sendFrame(frame); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// roomRegistry 负责命名空间房间的生命周期。
type roomRegistry struct {
	namespace *Namespace
	mu        sync.RWMutex
	rooms     map[string]*Room
}

func newRoomRegistry(ns *Namespace) *roomRegistry {
	return &roomRegistry{
		namespace: ns,
		rooms:     make(map[string]*Room),
	}
}

func (r *roomRegistry) GetOrCreate(name string) *Room {
	if name == "" {
		return nil
	}
	r.mu.Lock()
	room := r.rooms[name]
	if room == nil {
		room = newRoom(r.namespace, name)
		r.rooms[name] = room
	}
	r.mu.Unlock()
	return room
}

func (r *roomRegistry) Join(name string, conn *Conn) error {
	room := r.GetOrCreate(name)
	if room == nil {
		return ErrNoSuchRoom
	}
	room.add(conn)
	conn.addRoom(name)
	return nil
}

func (r *roomRegistry) Leave(name string, conn *Conn) {
	if name == "" || conn == nil {
		return
	}
	r.mu.RLock()
	room := r.rooms[name]
	r.mu.RUnlock()
	if room == nil {
		return
	}
	empty := room.remove(conn.id)
	conn.removeRoom(name)
	if empty {
		r.mu.Lock()
		if current, ok := r.rooms[name]; ok && current == room {
			delete(r.rooms, name)
		}
		r.mu.Unlock()
	}
}

func (r *roomRegistry) Select(names map[string]struct{}) []*Room {
	if len(names) == 0 {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	rooms := make([]*Room, 0, len(names))
	for name := range names {
		if room := r.rooms[name]; room != nil {
			rooms = append(rooms, room)
		}
	}
	return rooms
}
