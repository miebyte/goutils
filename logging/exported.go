// File:		exported.go
// Created by:	Hoven
// Created on:	2025-08-19
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/miebyte/goutils/logging/level"
)

var (
	logger *PrettyLogger
)

func init() {
	time.Local = time.FixedZone("CST", 8*3600)

	logger = NewPrettyLogger(os.Stdout)
}

func GetLogger() *PrettyLogger {
	return logger
}

func SetLogger(l *PrettyLogger) {
	logger = l
}

func WithSource(s bool) {
	logger.SetWithSource(s)
}

func SetFormatter(f Formatter) {
	logger.SetFormatter(f)
}

func IsDebug() bool {
	return logger.IsDebug()
}

func SetOutput(w io.Writer) {
	logger.SetOutput(w)
}

func Enable(l level.Level) {
	logger.Enable(l)
}

func Error(msg string) { logger.Error(msg) }
func Warn(msg string)  { logger.Warn(msg) }
func Debug(msg string) { logger.Debug(msg) }
func Info(msg string)  { logger.Info(msg) }

func Errorf(msg string, v ...any) {
	logger.Errorf(msg, v...)
}

func Warnf(msg string, v ...any) {
	logger.Warnf(msg, v...)
}

func Infof(msg string, v ...any) {
	logger.Infof(msg, v...)
}

func Debugf(msg string, v ...any) {
	logger.Debugf(msg, v...)
}

func Fatalf(msg string, v ...any) {
	logger.Fatalf(msg, v...)
}

func Infoc(ctx context.Context, msg string, v ...any) {
	logger.Infoc(ctx, msg, v...)
}

func Debugc(ctx context.Context, msg string, v ...any) {
	logger.Debugc(ctx, msg, v...)
}

func Warnc(ctx context.Context, msg string, v ...any) {
	logger.Warnc(ctx, msg, v...)
}

func Errorc(ctx context.Context, msg string, v ...any) {
	logger.Errorc(ctx, msg, v...)
}

func Fatalc(ctx context.Context, msg string, v ...any) {
	logger.Fatalc(ctx, msg, v...)
}

func PanicError(err error, v ...any) {
	if err == nil {
		return
	}

	var s string
	if len(v) > 0 {
		s = err.Error() + ":" + fmt.Sprint(v...)
	} else {
		s = err.Error()
	}

	logger.Error(s)
	panic(s)
}

func AddGlobalHook(hook Hook) {
	logger.AddHook(hook)
}
