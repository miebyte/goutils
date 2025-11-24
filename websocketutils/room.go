package websocketutils

import (
	cmap "github.com/orcaman/concurrent-map/v2"
)

var _ RoomAPI = (*Room)(nil)

type Room struct {
	name    string
	ns      *Namespace
	members cmap.ConcurrentMap[string, Conn]
}

func newRoom(ns *Namespace, name string) *Room {
	return &Room{
		name:    name,
		ns:      ns,
		members: cmap.New[Conn](),
	}
}

func (r *Room) Name() string {
	return r.name
}

// Broadcast 广播给房间成员
func (r *Room) Broadcast(event string, data any) {
	frame, err := encodeFrame(event, data)
	if err != nil {
		websocketLogger.Warnf("broadcast encode error: %v", err)
		return
	}

	for _, conn := range r.members.Items() {
		err = conn.SendFrame(frame)
		if err != nil {
			websocketLogger.Errorf("send frame error: %v", err)
		}
	}
}

// Add 添加连接到房间
func (r *Room) Add(conn Conn) {
	r.members.Set(conn.ID(), conn)
}

// Remove 从房间移除连接
func (r *Room) Remove(conn Conn) {
	r.members.Remove(conn.ID())
}

// Members 返回房间成员列表
func (r *Room) Members() []Conn {

	members := make([]Conn, 0, r.members.Count())
	for _, v := range r.members.Items() {
		members = append(members, v)
	}
	return members
}
