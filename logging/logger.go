package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/miebyte/goutils/logging/level"
)

type PrettyLogger struct {
	Out        io.Writer
	WithSource bool
	Formatter  Formatter
	module     string

	level     level.Level
	mu        MutexWrap
	hooks     LevelHook
	entryPool sync.Pool
}

type PrettyLoggerOption func(*PrettyLogger)

func WithModule(n string) PrettyLoggerOption {
	return func(pl *PrettyLogger) {
		pl.module = n
	}
}

func NewPrettyLogger(w io.Writer, opts ...PrettyLoggerOption) *PrettyLogger {
	l := &PrettyLogger{
		Out:        w,
		WithSource: true,
		Formatter:  new(TextFormatter),
		level:      level.LevelInfo,
		hooks:      make(LevelHook),
	}

	for _, opt := range opts {
		opt(l)
	}

	if l.module == "" {
		l.module = "DEFAULT"
	}

	return l
}

func (l *PrettyLogger) newEntry() *Entry {
	entry, ok := l.entryPool.Get().(*Entry)
	if ok {
		return entry
	}
	return NewEntry(l)
}

func (l *PrettyLogger) releaseEntry(entry *Entry) {
	entry.Data = map[string]any{}
	entry.Caller = nil
	entry.Buffer = nil
	entry.Ctx = nil
	l.entryPool.Put(entry)
}

func (l *PrettyLogger) IsLevelEnabled(level level.Level) bool {
	return level >= l.level
}

func (l *PrettyLogger) Enable(lev level.Level) {
	l.level = lev
}

func (l *PrettyLogger) SetNoLock() {
	l.mu.Disable()
}

func (l *PrettyLogger) SetWithSource(s bool) {
	logger.mu.Lock()
	defer logger.mu.Unlock()

	l.WithSource = s
}

func (l *PrettyLogger) SetFormatter(f Formatter) {
	logger.mu.Lock()
	defer logger.mu.Unlock()

	l.Formatter = f
}

func (l *PrettyLogger) IsDebug() bool {
	return l.level == level.LevelDebug
}

func (l *PrettyLogger) SetOutput(o io.Writer) {
	l.Out = o
}

func (l *PrettyLogger) AddHook(hook Hook) {
	for _, lev := range hook.Levels() {
		m, exists := l.hooks[lev]
		if !exists {
			m = make(map[string]Hook)
			l.hooks[lev] = m
		}

		m[hook.Name()] = hook
	}
}

func (l *PrettyLogger) log(lev level.Level, msg string) {
	if l.IsLevelEnabled(lev) {
		entry := l.newEntry()
		entry.Log(lev, msg)
		l.releaseEntry(entry)
	}
}

func (l *PrettyLogger) logf(lev level.Level, msg string, args ...any) {
	if l.IsLevelEnabled(lev) {
		entry := l.newEntry()
		entry.Log(lev, fmt.Sprintf(msg, args...))
		l.releaseEntry(entry)
	}
}

func (l *PrettyLogger) logc(ctx context.Context, lev level.Level, msg string, args ...any) {
	if l.IsLevelEnabled(lev) {
		entry := l.newEntry()
		entry.Ctx = ctx
		entry.Log(lev, fmt.Sprintf(msg, args...))
		l.releaseEntry(entry)
	}
}

func (l *PrettyLogger) Info(msg string) {
	l.log(level.LevelInfo, msg)
}

func (l *PrettyLogger) Debug(msg string) {
	l.log(level.LevelDebug, msg)
}

func (l *PrettyLogger) Warn(msg string) {
	l.log(level.LevelWarn, msg)
}

func (l *PrettyLogger) Error(msg string) {
	l.log(level.LevelError, msg)
}

func (l *PrettyLogger) Infoc(ctx context.Context, msg string, args ...any) {
	l.logc(ctx, level.LevelInfo, msg, args...)
}

func (l *PrettyLogger) Debugc(ctx context.Context, msg string, args ...any) {
	l.logc(ctx, level.LevelDebug, msg, args...)
}

func (l *PrettyLogger) Warnc(ctx context.Context, msg string, args ...any) {
	l.logc(ctx, level.LevelWarn, msg, args...)
}

func (l *PrettyLogger) Errorc(ctx context.Context, msg string, args ...any) {
	l.logc(ctx, level.LevelError, msg, args...)
}

func (l *PrettyLogger) Fatalc(ctx context.Context, msg string, args ...any) {
	l.logc(ctx, level.LevelError, msg, args...)
	os.Exit(1)
}

func (l *PrettyLogger) Infof(msg string, args ...any) {
	l.logf(level.LevelInfo, msg, args...)
}

func (l *PrettyLogger) Debugf(msg string, args ...any) {
	l.logf(level.LevelDebug, msg, args...)
}

func (l *PrettyLogger) Warnf(msg string, args ...any) {
	l.logf(level.LevelWarn, msg, args...)
}

func (l *PrettyLogger) Errorf(msg string, args ...any) {
	l.logf(level.LevelError, msg, args...)
}

func (l *PrettyLogger) Fatalf(msg string, args ...any) {
	l.logf(level.LevelError, msg, args...)
	os.Exit(1)
}

func (l *PrettyLogger) PanicError(err error, args ...any) {
	if err == nil {
		return
	}

	l.logf(level.LevelError, err.Error(), args...)
}
