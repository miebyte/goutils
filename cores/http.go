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
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/pkg/errors"
	"github.com/rs/cors"
)

const (
	healthCheckUrl = "/health_check"
)

func WithHttpCORS() ServiceOption {
	return func(cs *CoresService) {
		cs.httpCors = true
		innerlog.Logger.Debugf("Http enable CORS")
	}
}

func WithHttpHandler(pattern string, handler http.Handler) ServiceOption {
	return func(cs *CoresService) {
		if !strings.HasPrefix(pattern, "/") {
			pattern = "/" + pattern
		}

		if !strings.HasSuffix(pattern, "/") {
			pattern = pattern + "/"
		}

		cs.httpPatterns = append(cs.httpPatterns, pattern)
		innerlog.Logger.Debugf("Registered http endpoint. path=%s\n", pattern)
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

			handler := c.httpHandler
			if c.httpCors {
				handler = cors.AllowAll().Handler(handler)
			}

			if c.usePrometheus {
				handler = c.monitorHttp(handler)
			}

			c.httpServer = &http.Server{
				Handler:           handler,
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

func WithHttpServerConfig(config *HttpServerConfig) ServiceOption {
	return func(cs *CoresService) {
		cs.httpServerConfig = config
	}
}
