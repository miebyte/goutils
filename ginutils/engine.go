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

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/snail"
)

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
	engine.Use(LoggerMiddleware(), gin.Recovery())
	return engine.With(opts...)
}

type Router interface {
	Init(gin.IRouter)
}

type engineBuilder struct {
	engine     *gin.Engine
	middleware []gin.HandlerFunc
	routers    []*routerEntry
	groups     []*groupEntry
}

type routerEntry struct {
	router     Router
	middleware []gin.HandlerFunc
}

func NewRouter(router Router, middlewares ...gin.HandlerFunc) *routerEntry {
	return &routerEntry{
		router:     router,
		middleware: middlewares,
	}
}

type groupEntry struct {
	prefix     string
	middleware []gin.HandlerFunc
	routers    []*routerEntry
	groups     []*groupEntry
}

type EngineOption func(e *engineBuilder)

func NewServerHandler(opts ...EngineOption) *gin.Engine {
	e := &engineBuilder{
		engine: gin.New(withContextFallback()),
		middleware: []gin.HandlerFunc{
			LoggerMiddleware(),
			gin.Recovery(),
		},
		routers: make([]*routerEntry, 0),
		groups:  make([]*groupEntry, 0),
	}

	for _, opt := range opts {
		opt(e)
	}

	e.engine.Use(e.middleware...)

	for _, r := range e.routers {
		r.router.Init(e.engine)
	}

	for _, g := range e.groups {
		registerGroup(e.engine, g)
	}

	return e.engine
}

// registerGroup 递归注册 group 及其子 group
func registerGroup(parent gin.IRouter, g *groupEntry) {
	group := parent.Group(g.prefix, g.middleware...)

	for _, r := range g.routers {
		r.router.Init(group)
	}

	for _, subGroup := range g.groups {
		registerGroup(group, subGroup)
	}
}

func WithGroupRouters(group string, opts ...EngineOption) EngineOption {
	return func(e *engineBuilder) {
		builder := &engineBuilder{engine: e.engine}
		for _, opt := range opts {
			opt(builder)
		}

		e.groups = append(e.groups, &groupEntry{
			prefix:     group,
			middleware: builder.middleware,
			routers:    builder.routers,
			groups:     builder.groups,
		})
	}
}

func WithRootRouters(routers ...Router) EngineOption {
	return func(e *engineBuilder) {
		for _, router := range routers {
			e.routers = append(e.routers, NewRouter(router))
		}
	}
}

func WithRouters(basePath string, routers ...Router) EngineOption {
	return func(e *engineBuilder) {
		groupRouters := make([]*routerEntry, 0, len(routers))
		for _, router := range routers {
			groupRouters = append(groupRouters, NewRouter(router))
		}

		e.groups = append(e.groups, &groupEntry{
			prefix:  basePath,
			routers: groupRouters,
		})
	}
}

func WithMiddlewares(middlewares ...gin.HandlerFunc) EngineOption {
	return func(e *engineBuilder) {
		e.middleware = append(e.middleware, middlewares...)
	}
}

func WithLoggingRequest(header bool) EngineOption {
	return func(e *engineBuilder) {
		e.middleware = append(e.middleware, LoggingRequest(header))
	}

}

func WithReuseBody() EngineOption {
	return func(e *engineBuilder) {
		e.middleware = append(e.middleware, ReuseBody())
	}
}

// WithHiddenRoutesLog 隐藏路由启动日志
func WithHiddenRoutesLog() EngineOption {
	gin.DefaultWriter = io.Discard
	return func(e *engineBuilder) {}
}

func withContextFallback() gin.OptionFunc {
	return func(e *gin.Engine) {
		e.ContextWithFallback = true
	}
}
