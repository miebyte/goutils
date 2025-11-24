package websocketutils

import (
	"context"
	"encoding/json"
	"strings"
)

// Context 封装事件执行上下文。
type Context struct {
	ctx       context.Context
	conn      Conn
	namespace NamespaceAPI
	event     string
	data      json.RawMessage
}

func newEventContext(ctx context.Context, conn Conn, ns NamespaceAPI, event string, data json.RawMessage) *Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Context{
		ctx:       ctx,
		conn:      conn,
		namespace: ns,
		event:     event,
		data:      data,
	}
}

// Context 返回基础 context。
func (c *Context) Context() context.Context {
	if c == nil || c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// WithContext 替换内部 context。
func (c *Context) WithContext(ctx context.Context) {
	if ctx == nil {
		return
	}
	c.ctx = ctx
}

// Conn 返回关联连接。
func (c *Context) Conn() Conn {
	return c.conn
}

// Namespace 返回关联命名空间。
func (c *Context) Namespace() NamespaceAPI {
	return c.namespace
}

// Event 返回事件名。
func (c *Context) Event() string {
	return c.event
}

// Data 返回事件原始数据。
func (c *Context) Data() json.RawMessage {
	return c.data
}

// Scan 将数据反序列化到目标结构。
func (c *Context) Scan(v any) error {
	if len(c.data) == 0 || v == nil {
		return nil
	}
	return json.Unmarshal(c.data, v)
}

// To 创建房间上下文，便于向多个房间广播。
func (c *Context) To(room string) *RoomContext {
	if c == nil || c.namespace == nil {
		return nil
	}
	return newRoomContext(c.namespace).To(room)
}

// RoomContext 支持多房间广播。
type RoomContext struct {
	ns    NamespaceAPI
	rooms map[string]struct{}
}

func newRoomContext(ns NamespaceAPI) *RoomContext {
	return &RoomContext{
		ns: ns,
	}
}

// To 累积目标房间，返回当前上下文以便链式调用。
func (c *RoomContext) To(room string) *RoomContext {
	if c == nil {
		return nil
	}
	room = strings.TrimSpace(room)
	if room == "" {
		return c
	}
	if c.rooms == nil {
		c.rooms = make(map[string]struct{})
	}
	c.rooms[room] = struct{}{}
	return c
}

// Emit 广播事件到已选择的房间。
func (c *RoomContext) Emit(event string, payload any) error {
	targets := c.collectTargets(nil)
	var firstErr error
	for _, conn := range targets {
		if err := conn.Emit(event, payload); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			Logger().Errorf("room context emit failed namespace=%s conn=%s event=%s err=%v", c.namespaceName(), conn.ID(), event, err)
		}
	}
	return firstErr
}

// EmitExcept 广播事件到已选择的房间，排除指定连接。
func (c *RoomContext) EmitExcept(event string, payload any, conn Conn) error {
	targets := c.collectTargets(conn)
	var firstErr error
	for _, target := range targets {
		if err := target.Emit(event, payload); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			Logger().Errorf("room context emit except failed namespace=%s conn=%s event=%s err=%v", c.namespaceName(), target.ID(), event, err)
		}
	}
	return firstErr
}

func (c *RoomContext) collectTargets(except Conn) []Conn {
	if c == nil || c.ns == nil || len(c.rooms) == 0 {
		return nil
	}
	dedup := make(map[string]Conn)
	for room := range c.rooms {
		rm := c.ns.Room(room)
		if rm == nil {
			continue
		}
		for _, member := range rm.Members() {
			if member == nil {
				continue
			}
			if except != nil && member.ID() == except.ID() {
				continue
			}
			if _, exists := dedup[member.ID()]; exists {
				continue
			}
			dedup[member.ID()] = member
		}
	}
	if len(dedup) == 0 {
		return nil
	}
	targets := make([]Conn, 0, len(dedup))
	for _, conn := range dedup {
		targets = append(targets, conn)
	}
	return targets
}

func (c *RoomContext) namespaceName() string {
	if c == nil || c.ns == nil {
		return ""
	}
	return c.ns.Name()
}
