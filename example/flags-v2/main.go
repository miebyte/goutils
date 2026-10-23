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
	"github.com/miebyte/goutils/flags/v2"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/share"
)

type Config struct {
	Name string `json:"name"`
}

var (
	confFlag = flags.Struct("config", (*Config)(nil), "config")
)

func main() {
	// 在读取远程配置前固定服务名，运行期间不再修改。
	share.SetServiceName("test")
	flags.Parse(
		flags.WithConfigWatch(),
		flags.WithRemoteConfig(),
	)
	defer flags.Close()

	confLoader, err := confFlag()
	logging.PanicError(err)

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
					logging.Infoc(ctx, "%s", logging.Jsonify(confLoader.Load()))
				}
			}
		}),
	)

	// 长期 Worker 使用服务模式；一次性任务应改用 cores.Run。
	logging.PanicError(cores.Start(srv))
}
