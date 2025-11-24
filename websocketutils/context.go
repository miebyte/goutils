package websocketutils

import (
	"context"
	"encoding/json"
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
