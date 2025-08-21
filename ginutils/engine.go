// File:		engine.go
// Created by:	Hoven
// Created on:	2025-06-05
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package ginutils

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/snail"
)

var (
	anyMethods = []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodHead, http.MethodOptions, http.MethodDelete, http.MethodConnect,
		http.MethodTrace,
	}
)

type Router interface {
	Init(gin.IRouter)
}

func init() {
	snail.RegisterObject("ginModeSet", func() error {
		if logging.IsDebug() {
			gin.SetMode(gin.DebugMode)
		} else {
			gin.SetMode(gin.ReleaseMode)
		}
		return nil
	})
}

func Default(opts ...gin.OptionFunc) *gin.Engine {
	engine := gin.New()
	engine.Use(
		LoggerMiddleware(),
		gin.CustomRecovery(customRecoveryFn),
	)
	return engine.With(opts...)
}

// Option 同时支持 Engine 级别与分组级别的构建
type Option interface {
	applyEngine(*gin.Engine)
	applyGroup(*groupNode)
}

type groupNode struct {
	prefix      string
	middlewares []gin.HandlerFunc
	routes      []methodRoute
	routers     []Router
	children    []*groupNode
}

type methodRoute struct {
	method  string
	path    string
	handler gin.HandlerFunc
}

func (g *groupNode) applyEngine(e *gin.Engine) {
	g.register(e)
}

func (g *groupNode) applyGroup(parent *groupNode) {
	parent.children = append(parent.children, g)
}

func (g *groupNode) register(parent gin.IRouter) {
	r := parent
	if g.prefix != "" || len(g.middlewares) > 0 {
		r = parent.Group(g.prefix, g.middlewares...)
	}
	for _, rt := range g.routes {
		r.Handle(rt.method, rt.path, rt.handler)
	}
	for _, rh := range g.routers {
		if rh != nil {
			rh.Init(r)
		}
	}
	for _, child := range g.children {
		if child != nil {
			child.register(r)
		}
	}
}

// NewServerHandler 创建 Engine
func NewServerHandler(opts ...Option) *gin.Engine {
	engine := Default(withContextFallback())
	root := &groupNode{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}

		opt.applyGroup(root)

		if len(root.middlewares) > 0 {
			engine.Use(root.middlewares...)
			root.middlewares = nil
		}
	}
	root.register(engine)
	return engine
}

// WithGroupHandlers 定义一个可嵌套的路由分组
func WithGroupHandlers(opts ...Option) Option {
	g := &groupNode{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}

		opt.applyGroup(g)
	}
	return g
}

type groupOptionFunc func(*groupNode)

func (f groupOptionFunc) applyGroup(g *groupNode)   { f(g) }
func (f groupOptionFunc) applyEngine(e *gin.Engine) {}

// WithPrefix 设置分组前缀
func WithPrefix(prefix string) Option {
	return groupOptionFunc(func(g *groupNode) {
		g.prefix = prefix
	})
}

// WithMiddleware 添加分组中间件
func WithMiddleware(middlewares ...gin.HandlerFunc) Option {
	return groupOptionFunc(func(g *groupNode) {
		if len(middlewares) == 0 {
			return
		}
		g.middlewares = append(g.middlewares, middlewares...)
	})
}

// WithHandler 在分组中注册具体路由
func WithHandler(method string, path string, handler gin.HandlerFunc) Option {
	return groupOptionFunc(func(g *groupNode) {
		g.routes = append(g.routes, methodRoute{method: method, path: path, handler: handler})
	})
}

func WithAnyHandler(path string, handler gin.HandlerFunc) Option {
	return groupOptionFunc(func(g *groupNode) {
		for _, method := range anyMethods {
			g.routes = append(g.routes, methodRoute{method: method, path: path, handler: handler})
		}
	})
}

// WithRouterHandler 将实现了 Router 接口的路由器挂载到当前分组
func WithRouterHandler(routers ...Router) Option {
	return groupOptionFunc(func(g *groupNode) {
		if len(routers) == 0 {
			return
		}
		g.routers = append(g.routers, routers...)
	})
}

func WithLoggingRequest(header bool) gin.HandlerFunc {
	return LoggingRequest(header)
}

func WithHiddenRoutesLog() gin.HandlerFunc {
	gin.DefaultWriter = io.Discard
	return nil
}

func withContextFallback() gin.OptionFunc {
	return func(e *gin.Engine) {
		e.ContextWithFallback = true
	}
}
