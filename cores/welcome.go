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
	if c.listener != nil {
		innerlog.Logger.Infow(
			c.ctx, "Listening...",
			logging.String("Addr", c.listener.Addr().String()),
		)
	}

	if len(c.httpPattern) > 0 {
		_, port, _ := net.SplitHostPort(c.listenAddr)
		target := fmt.Sprintf("127.0.0.1:%s", port)

		for _, pattern := range c.httpPattern {
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
