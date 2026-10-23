// File:		welcome.go
// Created by:	Hoven
// Created on:	2025-04-02
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package cores

import (
	"fmt"
	"net"

	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/logging"
)

func (c *CoresService) welcome() {
	if c.internalHTTP.listener != nil {
		innerlog.Logger.Infow(
			c.ctx, "Internal HTTP listening...",
			logging.String("Addr", c.internalHTTP.listener.Addr().String()),
		)
	}
	if c.grpc != nil && c.grpc.listener != nil {
		innerlog.Logger.Infow(
			c.ctx, "Grpc listening...",
			logging.String("Addr", c.grpc.listener.Addr().String()),
		)
	}

	if srv := c.httpServer; srv != nil {
		_, port, _ := net.SplitHostPort(srv.listener.Addr().String())
		target := fmt.Sprintf("127.0.0.1:%s", port)

		for _, pattern := range srv.patterns {
			innerlog.Logger.Infow(
				c.ctx, "HttpHandler enabled.",
				logging.String("URL", fmt.Sprintf("http://%s%s", target, pattern.pattern)),
				logging.String("Handler", pattern.name),
			)
		}

	}

	if c.serviceAlias != "" {
		innerlog.Logger.Infoc(c.ctx, "Service: %s Started.\n", c.serviceAlias)
	} else {
		innerlog.Logger.Infoc(c.ctx, "Service Started.")
	}
}
