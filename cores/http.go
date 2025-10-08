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

		cs.httpPattern = pattern
		innerlog.Logger.Debugf("Registered http endpoint. path=%s\n", pattern)
		cs.httpMux.Handle(pattern, http.StripPrefix(strings.TrimSuffix(pattern, "/"), handler))
	}
}

func (c *CoresService) listenHttp(lst net.Listener) mountFn {
	return mountFn{
		fn: func(ctx context.Context) (err error) {

			handler := c.httpHandler
			if c.httpCors {
				handler = cors.AllowAll().Handler(handler)
			}

			c.httpServer = &http.Server{
				Handler:      handler,
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 10 * time.Second,
				IdleTimeout:  10 * time.Second,
			}

			err = c.httpServer.Serve(lst)
			if errors.Is(err, net.ErrClosed) {
				innerlog.Logger.Warnc(ctx, "listener is close. %v", err)
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
