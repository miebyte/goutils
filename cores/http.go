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
		cs.httpMux.Handle(so.prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, so.prefix+"/index.html", http.StatusFound)
		}))
		cs.httpMux.Handle(so.prefix+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r)
		}))
	}
}

type httpHandlerWithName interface {
	http.Handler
	Name() string
}

func WithHttpHandler(pattern string, handler http.Handler) ServiceOption {
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
		} else {
			handlerName = logging.GetStructName(handler)
		}

		cs.httpPattern = append(cs.httpPattern, httpPattern{
			pattern: pattern,
			name:    handlerName,
		})

		innerlog.Logger.Debugw(
			context.TODO(), "Registered http endpoint.",
			logging.String("pattern", pattern),
			logging.String("name", handlerName),
		)

		cs.httpMux.Handle(pattern, http.StripPrefix(strings.TrimSuffix(pattern, "/"), handler))
	}
}

func healthCheckApi(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (c *CoresService) listenHttp(lst net.Listener) mountFn {
	return mountFn{
		fn: func(ctx context.Context) (err error) {
			c.httpMux.Handle(healthCheckUrl, http.HandlerFunc(healthCheckApi))

			c.httpServer = &http.Server{
				Handler:           c.httpHandler,
				ReadTimeout:       c.httpServerConfig.ReadTimeout,
				ReadHeaderTimeout: c.httpServerConfig.ReadHeaderTimeout,
				WriteTimeout:      c.httpServerConfig.WriteTimeout,
				IdleTimeout:       c.httpServerConfig.IdleTimeout,
				MaxHeaderBytes:    c.httpServerConfig.MaxHeaderBytes,
			}
			err = c.httpServer.Serve(lst)
			if errors.Is(err, net.ErrClosed) || errors.Is(err, http.ErrServerClosed) {
				return nil
			} else if err != nil {
				innerlog.Logger.Errorc(ctx, "HttpListener serve error: %v\n", err)
				return err
			}
			return nil
		},
		name: "HttpListener",
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
