// File:		logger.go
// Created by:	Hoven
// Created on:	2025-04-03
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
	"github.com/miebyte/goutils/logging/slog"
	"gopkg.in/natefinch/lumberjack.v2"
)

type baseLogger interface {
	Enable(level.Level)
	IsDebug() bool
	SetOutput(io.Writer)
}

type FormatLogger interface {
	Infof(string, ...any)
	Debugf(string, ...any)
	Warnf(string, ...any)
	Errorf(string, ...any)
	Fatalf(string, ...any)
}

type ContextLogger interface {
	Infoc(context.Context, string, ...any)
	Debugc(context.Context, string, ...any)
	Warnc(context.Context, string, ...any)
	Errorc(context.Context, string, ...any)
	Fatalc(context.Context, string, ...any)
}

type Logger interface {
	baseLogger
	ContextLogger
	FormatLogger
	PanicError(error, ...any)
}

var (
	logger   Logger
	logLevel level.Level = level.LevelInfo
)

func init() {
	time.Local = time.FixedZone("CST", 8*3600)

	logger = slog.NewSlogPrettyLogger(
		os.Stdout,
		slog.WithCalldepth(6),
		slog.WithSource(),
	)
	logger.Enable(logLevel)
}

func SetSlog(opts ...slog.Option) {
	opts = append(opts, slog.WithCalldepth(6), slog.WithSource())
	logger = slog.New(opts...)
}

func GetLogger() Logger {
	return logger
}

func SetLogger(l Logger) {
	logger = l
	logger.Enable(logLevel)
}

func IsDebug() bool {
	return logger.IsDebug()
}

func EnableLogToFile(jackLog *LogConfig) {
	logger.SetOutput((*lumberjack.Logger)(jackLog))
}

func SetOutput(w io.Writer) {
	logger.SetOutput(w)
}

func Enable(l level.Level) {
	logger.Enable(l)
	logLevel = l
}

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

	logger.Errorc(context.TODO(), s)
	panic(s)
}
