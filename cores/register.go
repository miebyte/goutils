// File:		register.go
// Created by:	Hoven
// Created on:	2025-06-06
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package cores

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/miebyte/goutils/discover"
	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/pkg/errors"
)

const (
	CUSTOMER_REGISTER_ADDR = "GOUTILS_CUSTOMER_ADDR"
)

func (c *CoresService) registerService() mountFn {
	return mountFn{
		name:    "ServiceRegister",
		maxWait: time.Second * 10,
		fn: func(ctx context.Context) error {
			registerAddr := c.listenAddr

			customRegisterHost := os.Getenv(CUSTOMER_REGISTER_ADDR)
			if customRegisterHost != "" {
				hostSplit := strings.Split(customRegisterHost, ":")
				if len(hostSplit) == 2 {
					registerAddr = customRegisterHost
				} else {
					_, port, _ := net.SplitHostPort(c.listenAddr)
					registerAddr = fmt.Sprintf("%s:%s", customRegisterHost, port)
				}
			}

			err := discover.GetServiceFinder().RegisterServiceWithTags(c.serviceName, registerAddr, c.tags)
			if err != nil {
				innerlog.Logger.Errorc(ctx, "Register service(%s) failed. error: %v", c.serviceName, err)
				return errors.Wrap(err, "registerService")
			}

			innerlog.Logger.Infoc(ctx, "Register service(%s) success", c.serviceName)

			// Wait for terminate
			<-ctx.Done()

			discover.GetServiceFinder().Close()
			return nil
		},
	}
}
