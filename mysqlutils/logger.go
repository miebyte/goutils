// File:		logger.go
// Created by:	Hoven
// Created on:	2025-05-28
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package mysqlutils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/miebyte/goutils/logging"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

var (
	GormLogger logger.Interface
)

func init() {
	GormLogger = NewGormLogger(
		WithSlowThreshold(time.Second * 10),
	)
	GormLogger.LogMode(logger.Info)
}

type gormLogger struct {
	prefix                    string
	slowThreshold             time.Duration
	ignoreRecordNotFoundError bool
	logMod                    logger.LogLevel
	logger                    logging.Logger

	traceStr, traceErrStr, traceWarnStr string
}

type GormLoggerOption func(g *gormLogger)

func WithPrefix(prefix string) GormLoggerOption {
	return func(g *gormLogger) {
		g.prefix = prefix
	}
}

func WithIgnoreRecordNotFound() GormLoggerOption {
	return func(g *gormLogger) {
		g.ignoreRecordNotFoundError = true
	}
}

func WithSlowThreshold(dur time.Duration) GormLoggerOption {
	return func(g *gormLogger) {
		g.slowThreshold = dur
	}
}

func NewGormLogger(opts ...GormLoggerOption) *gormLogger {
	var (
		traceStr     = "%s\n[%.3fms] [rows:%v] %s"
		traceWarnStr = "%s %s\n[%.3fms] [rows:%v] %s"
		traceErrStr  = "%s %s\n[%.3fms] [rows:%v] %s"
	)

	logger := logging.NewPrettyLogger(
		os.Stdout,
		logging.WithModule("GORM"),
	)
	logger.SetWithSource(false)

	l := &gormLogger{
		logger:       logger,
		traceStr:     traceStr,
		traceWarnStr: traceWarnStr,
		traceErrStr:  traceErrStr,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

func (gl *gormLogger) wrapPrefix(ctx context.Context) context.Context {
	if gl.prefix != "" {
		ctx = logging.With(ctx, "[%v]", gl.prefix)
	}

	return ctx
}

func (gl *gormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *gl
	newLogger.logMod = level
	return &newLogger
}

func (gl *gormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	gl.logger.Infoc(gl.wrapPrefix(ctx), msg, data)
}

func (gl *gormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	gl.logger.Warnc(gl.wrapPrefix(ctx), msg, data)
}

func (gl *gormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	gl.logger.Errorc(gl.wrapPrefix(ctx), msg, data)
}

func (gl *gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	ctx = gl.wrapPrefix(ctx)
	elapsed := time.Since(begin)

	switch {
	case err != nil && (!errors.Is(err, logger.ErrRecordNotFound) || !gl.ignoreRecordNotFoundError):
		sql, rows := fc()
		if rows == -1 {
			gl.logger.Errorc(ctx, gl.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			gl.logger.Errorc(ctx, gl.traceErrStr, utils.FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	case elapsed > gl.slowThreshold && gl.slowThreshold != 0:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", gl.slowThreshold)
		if rows == -1 {
			gl.logger.Warnc(ctx, gl.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			gl.logger.Warnc(ctx, gl.traceWarnStr, utils.FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	case gl.logMod == logger.Info:
		sql, rows := fc()
		if rows == -1 {
			gl.logger.Infoc(ctx, gl.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			gl.logger.Infoc(ctx, gl.traceStr, utils.FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	}
}
