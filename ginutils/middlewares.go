// File:		middlewares.go
// Created by:	Hoven
// Created on:	2025-06-05
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package ginutils

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/logging/level"
)

var (
	Logger *logging.PrettyLogger
)

func init() {
	Logger = logging.NewPrettyLogger(os.Stdout, logging.WithModule("GINUTILS"))
	Logger.WithSource = false
	Logger.Enable(level.LevelDebug)
}

func LoggerMiddleware(loggers ...logging.Logger) gin.HandlerFunc {
	var logger logging.Logger
	if len(loggers) == 0 {
		logger = Logger
	} else {
		logger = loggers[0]
	}
	return func(c *gin.Context) {
		start := time.Now()

		clientIp := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		c.Next()

		statusCode := c.Writer.Status()
		spendTime := time.Since(start)

		var logFunc func(ctx context.Context, msg string, v ...any)
		switch {
		case statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices:
			logFunc = logger.Infoc
		case statusCode >= http.StatusMultipleChoices && statusCode < http.StatusBadRequest:
			logFunc = logger.Warnc
		case statusCode >= http.StatusBadRequest && statusCode <= http.StatusNetworkAuthenticationRequired:
			logFunc = logger.Warnc
		default:
			logFunc = logger.Errorc
		}

		args := []any{
			statusCode,
			method,
			path,
			spendTime,
			clientIp,
		}

		requestId, exists := c.Get("RequestID")
		if exists {
			args = append(args, requestId)
			logFunc(c, logMsg+" request_id=%s", args...)
			return
		}

		logFunc(c, logMsg, args...)
	}
}

// logMsg 访问日志模板：状态码、方法、路径在前，便于扫描。
var logMsg = "%3d %-7s %s elapsed=%v ip=%s"

func customRecoveryFn(c *gin.Context, err any) {
	Logger.Errorf(
		"[GinRecover] panic error: %v. path=%s url=%s method=%s host=%s ip=%s",
		err, c.Request.URL.Path, c.Request.URL, c.Request.Method, c.Request.Host, c.ClientIP(),
	)
	ReturnError(c, "System Error")
	c.Abort()
}
