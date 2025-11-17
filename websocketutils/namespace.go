package websocketutils

import "sync"

// Namespace 管理命名空间的事件与房间
type Namespace struct {
	name   string
	server *Server

	mu            sync.RWMutex
	clients       map[*Client]struct{}
	rooms         map[string]map[*Client]struct{}
	eventHandlers map[string]EventHandler
}

// On 注册命名空间事件处理器
func (ns *Namespace) On(event string, handler EventHandler) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.eventHandlers[event] = handler
}

// Emit 向命名空间所有客户端广播
func (ns *Namespace) Emit(event string, payload any) error {
	return ns.EmitExcept(event, payload, nil)
}

// EmitExcept 向命名空间广播并排除指定客户端
func (ns *Namespace) EmitExcept(event string, payload any, exclude *Client) error {
	msg, err := encodeMessage(MessageTypeEvent, ns.name, event, payload, "")
	if err != nil {
		return err
	}
	targets := ns.snapshotClients(exclude)
	for _, client := range targets {
		client.enqueue(msg)
	}
	return nil
}

// EmitToRoom 向命名空间房间广播
func (ns *Namespace) EmitToRoom(room, event string, payload any) error {
	return ns.EmitToRoomExcept(room, event, payload, nil)
}

// EmitToRoomExcept 向房间广播并排除指定客户端
func (ns *Namespace) EmitToRoomExcept(room, event string, payload any, exclude *Client) error {
	msg, err := encodeMessage(MessageTypeEvent, ns.name, event, payload, "")
	if err != nil {
		return err
	}
	targets := ns.snapshotRoom(room, exclude)
	for _, client := range targets {
		client.enqueue(msg)
	}
	return nil
}

func (ns *Namespace) dispatchEvent(c *Client, msg *WireMessage) {
	if msg.Event == "" {
		return
	}
	ns.mu.RLock()
	handler := ns.eventHandlers[msg.Event]
	ns.mu.RUnlock()
	if handler == nil {
		return
	}
	var ack func(any) error
	if msg.AckID != "" {
		ack = func(payload any) error {
			resp, err := encodeMessage(MessageTypeAck, ns.name, msg.Event, payload, msg.AckID)
			if err != nil {
				return err
			}
			return c.enqueue(resp)
		}
	}
	handler(c, &Event{Namespace: ns.name, Name: msg.Event, Data: msg.Data, Ack: ack})
}

func newNamespace(server *Server, name string) *Namespace {
	return &Namespace{
		name:          name,
		server:        server,
		clients:       make(map[*Client]struct{}),
		rooms:         make(map[string]map[*Client]struct{}),
		eventHandlers: make(map[string]EventHandler),
	}
}

func (ns *Namespace) addClient(c *Client) {
	ns.mu.Lock()
	if _, ok := ns.clients[c]; !ok {
		ns.clients[c] = struct{}{}
	}
	ns.mu.Unlock()
}

func (ns *Namespace) removeClient(c *Client) {
	ns.mu.Lock()
	delete(ns.clients, c)
	for room, members := range ns.rooms {
		if _, ok := members[c]; ok {
			delete(members, c)
			if len(members) == 0 {
				delete(ns.rooms, room)
			}
		}
	}
	ns.mu.Unlock()
}

func (ns *Namespace) joinRoom(c *Client, room string) {
	ns.mu.Lock()
	members := ns.rooms[room]
	if members == nil {
		members = make(map[*Client]struct{})
		ns.rooms[room] = members
	}
	members[c] = struct{}{}
	ns.mu.Unlock()
}

func (ns *Namespace) leaveRoom(c *Client, room string) {
	ns.mu.Lock()
	if members, ok := ns.rooms[room]; ok {
		delete(members, c)
		if len(members) == 0 {
			delete(ns.rooms, room)
		}
	}
	ns.mu.Unlock()
}

func (ns *Namespace) snapshotClients(exclude *Client) []*Client {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if len(ns.clients) == 0 {
		return nil
	}
	clients := make([]*Client, 0, len(ns.clients))
	for c := range ns.clients {
		if c == exclude {
			continue
		}
		clients = append(clients, c)
	}
	return clients
}

func (ns *Namespace) snapshotRoom(room string, exclude *Client) []*Client {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	members := ns.rooms[room]
	if len(members) == 0 {
		return nil
	}
	clients := make([]*Client, 0, len(members))
	for c := range members {
		if c == exclude {
			continue
		}
		clients = append(clients, c)
	}
	return clients
}
