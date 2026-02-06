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
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/logging/level"
)

const maxBodyLen = 1024

var (
	Logger *logging.PrettyLogger
)

func init() {
	Logger = logging.NewPrettyLogger(os.Stdout, logging.WithModule("GINLIBS"))
	Logger.WithSource = false
	Logger.Enable(level.LevelDebug)
}

// LoggingRequest 打印请求体
func LoggingRequest(header bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		msgTmp := "incoming http request Method=%s Url=%s Body=%s"
		args := []any{c.Request.Method, c.Request.URL.RequestURI(), requestBody(c)}

		if header {
			msgTmp += " Header=%s"
			args = append(args, c.Request.Header)
		}

		Logger.Infoc(c, msgTmp, args...)
		c.Next()
	}
}

func requestBody(c *gin.Context) string {
	if c.Request.Body == nil || c.Request.Body == http.NoBody {
		return ""
	}
	bodyData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logging.Errorc(c.Request.Context(), "[requestBody] read req body err: %v", err)
		return ""
	}
	_ = c.Request.Body.Close()
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyData))

	var buf bytes.Buffer
	if err := json.Compact(&buf, bodyData); err != nil {
		logging.Errorc(c.Request.Context(), "[requestBody] json.Compact body err: %v", err)
		return ""
	}

	return string(buf.Bytes()[:min(len(bodyData), maxBodyLen)])
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
			path,
			statusCode,
			spendTime,
			clientIp,
			method,
		}

		requestId, exists := c.Get("RequestID")
		if exists {
			args = append(args, requestId)
			logFunc(c, logMsg+" RequestID=%s", args...)
			return
		}

		logFunc(c, logMsg, args...)
	}
}

var (
	logMsg = "Handle Path: %s StatusCode=%v Elapse=%v Host=%s Method=%s"
)

func customRecoveryFn(c *gin.Context, err any) {
	Logger.Errorf(
		"[GinRecover] panic error: %v. path=%s url=%s method=%s host=%s ip=%s",
		err, c.Request.URL.Path, c.Request.URL, c.Request.Method, c.Request.Host, c.ClientIP(),
	)
	ReturnError(c, "System Error")

}
