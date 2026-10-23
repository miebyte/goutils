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
	"github.com/soheilhy/cmux"
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

// HttpHandler 表示一组业务路由前缀和处理器。
// 请求会先去掉 ApiPrefix，再交给 Handler；空前缀等同于根路径。
type HttpHandler struct {
	ApiPrefix string
	Handler   http.Handler
}

// WithHttpHandler 挂载多组业务路由，监听地址统一由 WithServiceAddress 指定。
// 重复调用时追加路由；HTTP 与 gRPC 共用业务端口，管理 HTTP 保持独立。
func WithHttpHandler(handlers ...HttpHandler) ServiceOption {
	return func(cs *CoresService) {
		if cs.httpServer == nil {
			cs.httpServer = &httpServer{mux: http.NewServeMux()}
		}
		if len(handlers) == 0 {
			cs.configErr = errors.New("at least one HTTP handler is required")
			return
		}

		for _, entry := range handlers {
			pattern := entry.ApiPrefix
			// ApiPrefix 是字面路径前缀，不接受 ServeMux 的通配符和方法语法。
			// 提前返回配置错误，避免批量挂载时由 ServeMux.Handle 直接 panic。
			if strings.ContainsAny(pattern, "{} \t\r\n") {
				cs.configErr = errors.New("HTTP API prefix must be a literal path prefix")
				return
			}
			if !strings.HasPrefix(pattern, "/") {
				pattern = "/" + pattern
			}
			if !strings.HasSuffix(pattern, "/") {
				pattern += "/"
			}

			var handlerName string
			if namedHandler, ok := entry.Handler.(httpHandlerWithName); ok {
				handlerName = namedHandler.Name()
			} else if entry.Handler != nil {
				handlerName = logging.GetStructName(entry.Handler)
			}
			cs.httpServer.patterns = append(cs.httpServer.patterns, httpPattern{
				pattern: pattern,
				name:    handlerName,
				handler: entry.Handler,
			})
			innerlog.Logger.Debugw(
				context.TODO(), "Registered http endpoint.",
				logging.String("pattern", pattern),
				logging.String("name", handlerName),
			)
		}
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
			if errors.Is(err, http.ErrServerClosed) || (c.stopping.Load() &&
				(errors.Is(err, net.ErrClosed) || errors.Is(err, cmux.ErrServerClosed) || errors.Is(err, cmux.ErrListenerClosed))) {
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
