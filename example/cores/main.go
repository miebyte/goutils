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
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/cores"
	"github.com/miebyte/goutils/flags"
	"github.com/miebyte/goutils/ginutils"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/share"
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
	port            = flags.Int("port", 8080, "HTTP port")
	internalAddress = flags.String("internal-address", ":6060", "管理 HTTP 监听地址")
	configFlag      = flags.Struct("config", (*Config)(nil), "server config")
	extraConfFlag   = flags.Struct("extra", (*ExtraConfig)(nil), "extra config")
)

func main() {
	share.SetServiceName("cores-example")
	flags.Parse()

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
		// 管理端口默认使用 :6060；本地同时启动多个服务时可分别覆盖。
		cores.WithInternalHTTPAddress(internalAddress()),
		cores.WithPprof(),
		cores.WithServiceAddress(fmt.Sprintf(":%d", port())),
		// 多个路由组共享一个业务端口，挂载前缀在进入 Handler 前会被移除。
		cores.WithHttpHandler(
			cores.HttpHandler{ApiPrefix: "/api", Handler: app},
			cores.HttpHandler{ApiPrefix: "/other", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprintln(w, "other handler")
			})},
		),
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
	logging.PanicError(cores.Start(srv))
}
