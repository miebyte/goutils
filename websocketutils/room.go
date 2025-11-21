package websocketutils

import "sync"

// Room 代表命名空间中的房间。
type Room struct {
	name      string
	namespace *Namespace
	mu        sync.RWMutex
	members   map[string]connection
}

func newRoom(ns *Namespace, name string) *Room {
	return &Room{
		name:      name,
		namespace: ns,
		members:   make(map[string]connection),
	}
}

// Name 返回房间名称。
func (r *Room) Name() string {
	return r.name
}

// Emit 向房间内所有连接广播事件。
func (r *Room) Emit(event string, payload any) error {
	return r.emit(event, payload, "")
}

// EmitExcept 在广播时排除指定连接。
func (r *Room) EmitExcept(event string, payload any, socket Socket) error {
	if socket == nil {
		return r.emit(event, payload, "")
	}
	return r.emit(event, payload, socket.ID())
}

func (r *Room) emit(event string, payload any, exclude string) error {
	data, err := encodeFrame(event, payload)
	if err != nil {
		return err
	}

	return r.namespace.hub.Deliver(data, r.receivers(exclude))
}

func (r *Room) add(conn connection) {
	r.mu.Lock()
	r.members[conn.ID()] = conn
	r.mu.Unlock()
}

func (r *Room) remove(connID string) bool {
	r.mu.Lock()
	delete(r.members, connID)
	empty := len(r.members) == 0
	r.mu.Unlock()
	return empty
}

func (r *Room) receivers(exclude string) []connection {
	r.mu.RLock()
	defer r.mu.RUnlock()
	receivers := make([]connection, 0, len(r.members))
	for id, conn := range r.members {
		if exclude != "" && id == exclude {
			continue
		}
		receivers = append(receivers, conn)
	}
	return receivers
}
