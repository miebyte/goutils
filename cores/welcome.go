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
)

func (c *CoresService) welcome() {
	if c.listener != nil {
		innerlog.Logger.Infof("Listening... Addr=%v\n", c.listener.Addr().String())
	}

	if c.httpPattern != "" {
		_, port, _ := net.SplitHostPort(c.listenAddr)
		target := fmt.Sprintf("127.0.0.1:%s", port)
		innerlog.Logger.Infof("HttpHandler enabled. URL=%s\n", fmt.Sprintf("http://%s%s", target, c.httpPattern))
	}

	if c.serviceName != "" {
		innerlog.Logger.Infof("Service: %s Tags: %v Started.\n", c.serviceName, c.tags)
	} else {
		innerlog.Logger.Infof("Service Started.")
	}
}
