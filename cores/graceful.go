// File:		graceful.go
// Created by:	Hoven
// Created on:	2025-04-02
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package cores

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/miebyte/goutils/logging"
	"github.com/pkg/errors"
)

func (c *CoresService) gracefulKill() mountFn {
	return mountFn{
		name:    "GracefulKill",
		maxWait: time.Second * 3,
		fn: func(ctx context.Context) error {
			ch := make(chan os.Signal, 1)
			signal.Notify(
				ch,
				os.Interrupt,
				syscall.SIGHUP,
				syscall.SIGINT,
				syscall.SIGTERM,
				syscall.SIGQUIT,
			)

			select {
			case sg := <-ch:
				logging.Infoc(ctx, "Graceful stopping service... Signal: %s", sg)
				c.cancel()

				if c.listener != nil {
					_ = c.listener.Close()
					logging.Infoc(ctx, "Graceful stopped listener")
				}

				logging.Infoc(ctx, "Graceful stopped service successfully")
				return errors.Errorf("Signal: %s", sg.String())
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
}
