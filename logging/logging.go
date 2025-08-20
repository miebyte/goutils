package logging

import (
	"context"
	"io"

	"github.com/miebyte/goutils/logging/level"
)

type baseLogger interface {
	Enable(level.Level)
	IsDebug() bool
	AddHook(hook Hook)
	SetOutput(io.Writer)
	SetNoLock()
	SetFormatter(f Formatter)
	SetWithSource(s bool)
}

type MessageLogger interface {
	Info(string)
	Debug(string)
	Warn(string)
	Error(string)
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
	MessageLogger
	ContextLogger
	FormatLogger
	PanicError(error, ...any)
}
