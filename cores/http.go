// File:		http.go
// Created by:	Hoven
// Created on:	2025-04-02
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package cores

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/logging"
	"github.com/pkg/errors"
)

const (
	healthCheckUrl = "/health_check"
)

// Deprecated: WithHttpCORS is deprecated
// use cors middleware in web service by yourself
func WithHttpCORS() ServiceOption {
	return func(cs *CoresService) {
		cs.httpCors = false
	}
}

type swaggerOption struct {
	prefix string
}

type swaggerOptionFn func(*swaggerOption)

func WithOpenapiPrefix(prefix string) swaggerOptionFn {
	return func(so *swaggerOption) {
		if prefix == "/" {
			innerlog.Logger.PanicError(fmt.Errorf("swagger openapi url not allow '/'"))
		}

		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}

		prefix = strings.TrimSuffix(prefix, "/")
		so.prefix = prefix
	}
}

func WithSwagger(handler http.Handler, opts ...swaggerOptionFn) ServiceOption {
	so := new(swaggerOption)
	for _, opt := range opts {
		opt(so)
	}

	if so.prefix == "" {
		so.prefix = "/docs"
	}

	return func(cs *CoresService) {
		cs.internalHTTP.mux.Handle(so.prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, so.prefix+"/index.html", http.StatusFound)
		}))
		cs.internalHTTP.mux.Handle(so.prefix+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r)
		}))
	}
}

type httpHandlerWithName interface {
	http.Handler
	Name() string
}

type httpServer struct {
	addr     string
	mux      *http.ServeMux
	patterns []httpPattern
	listener net.Listener
	server   *http.Server
}

// WithHttpHandler mounts handler on a business HTTP address, separate from Start's
// internal HTTP server. Handlers with the same address share a listener and mux.
func WithHttpHandler(addr string, pattern string, handler http.Handler) ServiceOption {
	return func(cs *CoresService) {
		if !strings.HasPrefix(pattern, "/") {
			pattern = "/" + pattern
		}

		if !strings.HasSuffix(pattern, "/") {
			pattern = pattern + "/"
		}

		var handlerName string
		namedHandler, ok := handler.(httpHandlerWithName)
		if ok {
			handlerName = namedHandler.Name()
		} else if handler != nil {
			handlerName = logging.GetStructName(handler)
		}

		var srv *httpServer
		for _, existing := range cs.httpServers {
			if existing.addr == addr {
				srv = existing
				break
			}
		}
		if srv == nil {
			srv = &httpServer{addr: addr, mux: http.NewServeMux()}
			cs.httpServers = append(cs.httpServers, srv)
		}
		srv.patterns = append(srv.patterns, httpPattern{
			pattern: pattern,
			name:    handlerName,
			handler: handler,
		})

		innerlog.Logger.Debugw(
			context.TODO(), "Registered http endpoint.",
			logging.String("pattern", pattern),
			logging.String("addr", addr),
			logging.String("name", handlerName),
		)
	}
}

func healthCheckApi(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (c *CoresService) listenHttp(srv *httpServer, name string) mountFn {
	srv.server = &http.Server{
		Handler:           srv.mux,
		ReadTimeout:       c.httpServerConfig.ReadTimeout,
		ReadHeaderTimeout: c.httpServerConfig.ReadHeaderTimeout,
		WriteTimeout:      c.httpServerConfig.WriteTimeout,
		IdleTimeout:       c.httpServerConfig.IdleTimeout,
		MaxHeaderBytes:    c.httpServerConfig.MaxHeaderBytes,
	}

	return mountFn{
		fn: func(ctx context.Context) (err error) {
			err = srv.server.Serve(srv.listener)
			if errors.Is(err, http.ErrServerClosed) || (c.stopping.Load() && errors.Is(err, net.ErrClosed)) {
				return nil
			} else if err != nil {
				innerlog.Logger.Errorc(ctx, "HttpListener serve error: %v\n", err)
				return err
			}
			return nil
		},
		name: name,
	}
}

type HttpServerConfig struct {
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}

func (hc *HttpServerConfig) SetDefault() {
	if hc.ReadTimeout == 0 {
		hc.ReadTimeout = defaultHttpServerConfig.ReadTimeout
	}

	if hc.ReadHeaderTimeout == 0 {
		hc.ReadHeaderTimeout = defaultHttpServerConfig.ReadHeaderTimeout
	}

	if hc.WriteTimeout == 0 {
		hc.WriteTimeout = defaultHttpServerConfig.WriteTimeout
	}

	if hc.IdleTimeout == 0 {
		hc.IdleTimeout = defaultHttpServerConfig.IdleTimeout
	}

	if hc.MaxHeaderBytes == 0 {
		hc.MaxHeaderBytes = defaultHttpServerConfig.MaxHeaderBytes
	}
}

var (
	defaultHttpServerConfig = &HttpServerConfig{
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       300 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB,
	}
)
