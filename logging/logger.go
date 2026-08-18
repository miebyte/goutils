package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/miebyte/goutils/logging/level"
	"github.com/miebyte/goutils/utils"
)

var _ Logger = (*PrettyLogger)(nil)

type PrettyLogger struct {
	Out        io.Writer
	WithSource bool
	Formatter  Formatter
	module     string

	level     level.Level
	mu        MutexWrap
	hooksMu   RWMutexWrap
	hooks     LevelHook
	entryPool sync.Pool
}

type PrettyLoggerOption func(*PrettyLogger)

func WithModule(n string) PrettyLoggerOption {
	return func(pl *PrettyLogger) {
		pl.module = n
	}
}

func WithEnableSource(b bool) PrettyLoggerOption {
	return func(pl *PrettyLogger) {
		pl.WithSource = b
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
	if entry.Data == nil {
		entry.Data = make(CtxFields, 6)
	} else {
		clear(entry.Data)
	}
	entry.Buffer = nil
	entry.Ctx = nil
	entry.Source = ""
	entry.WithoutMasking = false
	clear(entry.Fields)
	entry.Fields = entry.Fields[:0]

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

func (l *PrettyLogger) logw(ctx context.Context, lev level.Level, msg string, args ...Field) {
	if l.IsLevelEnabled(lev) {
		entry := l.newEntry()
		entry.Ctx = ctx
		entry.Fields = append(entry.Fields[:0], args...)
		entry.Log(lev, msg)
		l.releaseEntry(entry)
	}
}

func (l *PrettyLogger) logs(ctx context.Context, lev level.Level, msg string, args ...any) {
	if l.IsLevelEnabled(lev) {
		entry := l.newEntry()
		entry.Ctx = ctx
		entry.Fields = append(entry.Fields[:0], l.parseSugaredArgs(args)...)
		entry.Log(lev, msg)
		l.releaseEntry(entry)
	}
}

func (l *PrettyLogger) parseSugaredArgs(args []any) []Field {
	if len(args) == 0 {
		return nil
	}

	fields := make([]Field, 0, len(args)/2)

	for key, value := range utils.Pairwise(args) {
		key, ok := key.(string)
		if !ok || key == "" {
			continue
		}

		fields = append(fields, Any(key, value))
	}

	return fields
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

// Infow 输出带上下文与结构化字段的 info 日志。
func (l *PrettyLogger) Infow(ctx context.Context, msg string, args ...Field) {
	l.logw(ctx, level.LevelInfo, msg, args...)
}

// Debugw 输出带上下文与结构化字段的 debug 日志。
func (l *PrettyLogger) Debugw(ctx context.Context, msg string, args ...Field) {
	l.logw(ctx, level.LevelDebug, msg, args...)
}

// Warnw 输出带上下文与结构化字段的 warn 日志。
func (l *PrettyLogger) Warnw(ctx context.Context, msg string, args ...Field) {
	l.logw(ctx, level.LevelWarn, msg, args...)
}

// Errorw 输出带上下文与结构化字段的 error 日志。
func (l *PrettyLogger) Errorw(ctx context.Context, msg string, args ...Field) {
	l.logw(ctx, level.LevelError, msg, args...)
}

// Fatalw 输出带上下文与结构化字段的 fatal 日志并退出进程。
func (l *PrettyLogger) Fatalw(ctx context.Context, msg string, args ...Field) {
	l.logw(ctx, level.LevelError, msg, args...)
	os.Exit(1)
}

// Infos 输出带上下文与 key/value 字段的 info 日志。
func (l *PrettyLogger) Infos(ctx context.Context, msg string, args ...any) {
	l.logs(ctx, level.LevelInfo, msg, args...)
}

// Debugs 输出带上下文与 key/value 字段的 debug 日志。
func (l *PrettyLogger) Debugs(ctx context.Context, msg string, args ...any) {
	l.logs(ctx, level.LevelDebug, msg, args...)
}

// Warns 输出带上下文与 key/value 字段的 warn 日志。
func (l *PrettyLogger) Warns(ctx context.Context, msg string, args ...any) {
	l.logs(ctx, level.LevelWarn, msg, args...)
}

// Errors 输出带上下文与 key/value 字段的 error 日志。
func (l *PrettyLogger) Errors(ctx context.Context, msg string, args ...any) {
	l.logs(ctx, level.LevelError, msg, args...)
}

// Fatals 输出带上下文与 key/value 字段的 fatal 日志并退出进程。
func (l *PrettyLogger) Fatals(ctx context.Context, msg string, args ...any) {
	l.logs(ctx, level.LevelError, msg, args...)
	os.Exit(1)
}

// PanicError 在错误非空时记录错误日志并触发 panic。
func (l *PrettyLogger) PanicError(err error, args ...any) {
	if err == nil {
		return
	}

	var s string
	if len(args) > 0 {
		s = err.Error() + ":" + fmt.Sprint(args...)
	} else {
		s = err.Error()
	}

	l.log(level.LevelError, s)
	panic(s)
}

func (l *PrettyLogger) Log(lev level.Level, msg string, opt *LogOption) {
	if l.IsLevelEnabled(lev) {
		entry := l.newEntry()
		entryApplyOptions(entry, opt)
		entry.Log(lev, msg)
		l.releaseEntry(entry)
	}
}

func (l *PrettyLogger) Logc(ctx context.Context, lev level.Level, msg string, opt *LogOption) {
	if l.IsLevelEnabled(lev) {
		entry := l.newEntry()
		entry.Ctx = ctx
		entryApplyOptions(entry, opt)
		entry.Log(lev, msg)
		l.releaseEntry(entry)
	}
}

func entryApplyOptions(entry *Entry, opt *LogOption) {
	if opt == nil {
		return
	}

	if opt.WithoutMasking {
		entry.WithoutMasking = true
	}
}
