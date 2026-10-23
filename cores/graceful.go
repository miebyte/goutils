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

	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func (c *CoresService) gracefulKill() mountFn {
	return mountFn{
		name: "GracefulKill",
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
			defer signal.Stop(ch)

			select {
			case sg := <-ch:
				innerlog.Logger.Infoc(ctx, "Graceful stopping service... Signal: %s", sg)
				return errors.Errorf("Signal: %s", sg.String())
			case <-ctx.Done():
				return nil
			}
		},
	}
}

func (c *CoresService) shutdownServers() error {
	c.stopping.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), defaultMaxWait)
	defer cancel()

	var grp errgroup.Group
	for _, srv := range append([]*httpServer{c.internalHTTP}, c.httpServers...) {
		if srv.server != nil {
			grp.Go(func() error {
				if err := srv.server.Shutdown(ctx); err != nil {
					_ = srv.server.Close()
					return errors.Wrapf(err, "http server shutdown at %s", srv.listener.Addr().String())
				}
				return nil
			})
		}
	}
	if c.grpc != nil {
		grp.Go(func() error {
			done := make(chan struct{})
			go func() {
				c.grpc.server.GracefulStop()
				close(done)
			}()

			select {
			case <-done:
				return nil
			case <-ctx.Done():
				innerlog.Logger.Warnc(c.ctx, "Force closing grpc server after %v", defaultMaxWait)
				// Stop can also block behind GracefulStop or WaitForHandlers.
				go c.grpc.server.Stop()
				return errors.Wrap(ctx.Err(), "grpc server shutdown")
			}
		})
	}
	return grp.Wait()
}
