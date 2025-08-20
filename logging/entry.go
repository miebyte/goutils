package logging

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/miebyte/goutils/logging/level"
)

var (
	loggingPackage     string
	minimumCallerDepth int
	callerInitOnce     sync.Once
)

const (
	maximumCallerDepth int = 25
	defaultFrames      int = 4
)

type Fields map[string]any

func (f Fields) Clone() Fields {
	nf := make(Fields)
	maps.Copy(nf, f)
	return nf
}

type Entry struct {
	Ctx     context.Context
	Logger  *PrettyLogger
	Message string
	Data    Fields
	Time    time.Time
	Level   level.Level
	Caller  *runtime.Frame
	Buffer  *bytes.Buffer
}

func NewEntry(logger *PrettyLogger) *Entry {
	return &Entry{
		Logger: logger,
		Data:   make(Fields, 6),
	}
}

func (entry *Entry) fireHooks() {
	entry.Logger.mu.Lock()
	if entry.Logger.hooks.IsEmpty() {
		entry.Logger.mu.Unlock()
		return
	}

	tmpHooks := make(LevelHook, len(entry.Logger.hooks))
	maps.Copy(tmpHooks, entry.Logger.hooks)
	entry.Logger.mu.Unlock()

	err := tmpHooks.Fire(entry.Level, entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fire hook: %v\n", err)
	}
}

func (entry *Entry) write() {
	entry.Logger.mu.Lock()
	defer entry.Logger.mu.Unlock()

	serialized, err := entry.Logger.Formatter.Format(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to obtain reader, %v\n", err)
		return
	}

	if _, err := entry.Logger.Out.Write(serialized); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write to log, %v\n", err)
	}
}

func (entry *Entry) log() {
	buffer := bufferPool.Get()
	defer func() {
		entry.Buffer = nil
		buffer.Reset()
		bufferPool.Put(buffer)
	}()

	buffer.Reset()
	entry.Buffer = buffer
	entry.write()
}

func (entry *Entry) Log(lev level.Level, msg string) {
	entry.Time = time.Now()
	entry.Level = lev
	entry.Message = msg
	if entry.Logger.WithSource {
		entry.Caller = getCaller()
	}

	if entry.Ctx != nil {
		entry.Data = GetContextFields(entry.Ctx)
	} else {
		entry.Ctx = context.TODO()
	}
	entry.fireHooks()

	entry.log()
}

func getPackageName(f string) string {
	for {
		lastPeriod := strings.LastIndex(f, ".")
		lastSlash := strings.LastIndex(f, "/")
		if lastPeriod > lastSlash {
			f = f[:lastPeriod]
		} else {
			break
		}
	}

	return f
}

func getCaller() *runtime.Frame {
	callerInitOnce.Do(func() {
		pcs := make([]uintptr, maximumCallerDepth)
		_ = runtime.Callers(0, pcs)

		for i := range maximumCallerDepth {
			funcName := runtime.FuncForPC(pcs[i]).Name()
			if strings.Contains(funcName, "getCaller") {
				loggingPackage = getPackageName(funcName)
				break
			}
		}

		minimumCallerDepth = defaultFrames
	})

	pcs := make([]uintptr, maximumCallerDepth)
	depth := runtime.Callers(minimumCallerDepth, pcs)
	frames := runtime.CallersFrames(pcs[:depth])

	for f, again := frames.Next(); again; f, again = frames.Next() {
		pkg := getPackageName(f.Function)

		if pkg != loggingPackage {
			return &f
		}
	}

	return nil
}

func (entry *Entry) FieldsKVPairs() []any {
	kvPairs := make([]any, len(entry.Data)*2)
	for k, v := range maps.All(entry.Data) {
		kvPairs = append(kvPairs, k, v)
	}
	return kvPairs
}
