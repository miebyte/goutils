// File:		middlewares.go
// Created by:	Hoven
// Created on:	2025-06-05
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package ginutils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/logging"
)

func ReuseBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		buf := bytes.Buffer{}
		c.Request.Body = io.NopCloser(io.TeeReader(c.Request.Body, &buf))
		c.Next()
		c.Request.Body = io.NopCloser(&buf)
	}
}

const maxBodyLen = 1024

func LoggingRequest(header bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		fields := map[string]interface{}{
			"method": c.Request.Method,
			"uri":    c.Request.URL.RequestURI(),
			"remote": c.Request.RemoteAddr,
			"body":   requestBody(c),
		}
		if header {
			fields["header"] = c.Request.Header
		}
		logging.Infoc(ctx, "incoming http request: %+v", fields)
		c.Next()
	}
}

func requestBody(c *gin.Context) string {
	if c.Request.Body == nil || c.Request.Body == http.NoBody {
		return ""
	}
	bodyData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return fmt.Sprintf("read request body err: %s", err.Error())
	}
	_ = c.Request.Body.Close()
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyData))

	bodySize := len(bodyData)
	if bodySize > maxBodyLen {
		bodySize = maxBodyLen
	}
	return string(bodyData[:bodySize])
}

func LoggerMiddleware(loggers ...logging.Logger) gin.HandlerFunc {
	var logger logging.Logger
	if len(loggers) == 0 {
		logger = logging.NewPrettyLogger(os.Stdout, logging.WithModule("GINLIBS"))
		logger.SetWithSource(false)
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
			spendTime,
			clientIp,
			method,
			path,
		}

		logFunc(c, logMsg, args...)
	}
}

const (
	logMsg = "| %v | %13v | %15s | %4v | %#v"
)
