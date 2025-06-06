// File:		main.go
// Created by:	Hoven
// Created on:	2025-06-05
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package main

import (
	"context"
	"time"

	"github.com/miebyte/goutils/cores"
	"github.com/miebyte/goutils/flags"
	"github.com/miebyte/goutils/logging"
)

var (
	port = flags.Int("port", 0, "port")
)

func main() {
	flags.Parse()

	srv := cores.NewCores(
		// cores.WithWaitAllDone(),
		cores.WithWorker(func(ctx context.Context) error {
			ticket := time.NewTicker(time.Second * 3)
			defer ticket.Stop()

			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticket.C:
					logging.Infoc(ctx, "this is worker")
				}
			}
		}),
	)

	logging.PanicError(cores.Start(srv, port()))
}
