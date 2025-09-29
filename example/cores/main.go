// File:		main.go
// Created by:	Hoven
// Created on:	2025-04-15
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package main

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/cores"
	"github.com/miebyte/goutils/flags"
	"github.com/miebyte/goutils/ginutils"
	"github.com/miebyte/goutils/logging"
)

type Config struct {
	Province string         `json:"province"`
	City     string         `json:"city"`
	Extra    *ExtraConfig   `json:"extra"`
	DataMap  map[string]any `json:"data_map"`
}

func (c *Config) Reload() {
	// do reload func
	logging.Infof("config reload: %v", logging.Jsonify(c))
}

type ExtraConfig struct {
	Extra   string         `json:"extra"`
	IPS     []string       `json:"ips"`
	DataMap map[string]any `json:"data_map"`
}

var (
	port          = flags.Int("port", 8080, "port")
	configFlag    = flags.Struct("config", (*Config)(nil), "server config")
	extraConfFlag = flags.Struct("extra", (*ExtraConfig)(nil), "extra config")
)

func main() {
	flags.Parse(flags.WithConfigWatch())

	conf := new(Config)
	logging.PanicError(configFlag(conf))

	extraConf := new(ExtraConfig)
	logging.PanicError(extraConfFlag(extraConf))

	logging.Infof("conf: %v, extra conf: %v", logging.Jsonify(conf), logging.Jsonify(extraConf))

	app := ginutils.Default()

	app.GET("/users", func(c *gin.Context) {
		time.Sleep(time.Second * 10)
		ginutils.ReturnSuccess(c, "111")
	})

	app.GET("/error", func(c *gin.Context) {
		logging.Errorc(c, "this is error log")
		ginutils.ReturnSuccess(c, "success")
	})

	srv := cores.NewCores(
		cores.WithPprof(),
		cores.WithHttpHandler("/", app),
		cores.WithWorker(func(ctx context.Context) error {
			ticket := time.NewTicker(time.Second * 3)
			defer ticket.Stop()

			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticket.C:
				}

				logging.Infof("conf: %v", logging.Jsonify(conf))
				logging.Infof("extra conf: %v", logging.Jsonify(extraConf))
			}
		}),
	)
	logging.PanicError(cores.Start(srv, port()))
}
