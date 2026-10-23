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
	"github.com/miebyte/goutils/share"
)

func main() {
	share.SetServiceName("flags-example")
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

	// 长期 Worker 使用服务模式，默认提供 :6060/health_check 和 /ready。
	logging.PanicError(cores.Start(srv))
}
