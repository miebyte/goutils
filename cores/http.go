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

	"github.com/miebyte/goutils/logging"
	"github.com/pkg/errors"
)

func WithHttpCORS() ServiceOption {
	return func(cs *CoresService) {
		cs.httpCors = true
		logging.Debugf("Http enable CORS")
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
		logging.Debugf("Registered http endpoint. path=%s\n", pattern)
		cs.httpMux.Handle(pattern, http.StripPrefix(strings.TrimSuffix(pattern, "/"), handler))
	}
}

func (c *CoresService) listenHttp(lst net.Listener) mountFn {
	return mountFn{
		fn: func(ctx context.Context) (err error) {
			err = http.Serve(lst, c.httpHandler)
			if errors.Is(err, net.ErrClosed) {
				logging.Warnc(ctx, "listener is close. %v", err)
				return nil
			} else if err != nil {
				logging.Errorc(ctx, "HttpListener serve error: %v\n", err)
				return err
			}
			return nil
		},
		name: "HttpListener",
	}
}
