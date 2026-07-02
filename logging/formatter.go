// File:		formatter.go
// Created by:	Hoven
// Created on:	2025-08-19
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package logging

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-isatty"
	"github.com/miebyte/goutils/logging/level"
)

type Formatter interface {
	Format(*Entry) ([]byte, error)
}

type TextFormatter struct {
	isTerminal         bool
	levelTextMaxLength int
	terminalInitOnce   sync.Once

	DisableColors bool
	CompactMode   bool
}

const timestampFormat = "2006-01-02 15:04:05.000"

func checkIfTerminal(w io.Writer) bool {
	if _, exists := os.LookupEnv("NO_COLOR"); exists {
		return false
	}

	if w, ok := w.(*os.File); !ok || os.Getenv("TERM") == "dumb" ||
		(!isatty.IsTerminal(w.Fd()) && !isatty.IsCygwinTerminal(w.Fd())) {
		return false
	}

	return true
}

func (f *TextFormatter) isColored() bool {
	isColored := (f.isTerminal && (runtime.GOOS != "windows"))

	return isColored && !f.DisableColors
}

func (f *TextFormatter) init(entry *Entry) {
	if entry.Logger != nil {
		f.isTerminal = checkIfTerminal(entry.Logger.Out)
	}

	// Get the max length of the level text
	for _, level := range level.AllLevels {
		levelTextLength := utf8.RuneCount([]byte(level.String()))
		if levelTextLength > f.levelTextMaxLength {
			f.levelTextMaxLength = levelTextLength
		}
	}
}

func (f *TextFormatter) Format(e *Entry) ([]byte, error) {
	f.terminalInitOnce.Do(func() {
		f.init(e)
	})

	var buf *bytes.Buffer
	if e.Buffer != nil {
		buf = e.Buffer
	} else {
		buf = &bytes.Buffer{}
	}

	if f.CompactMode {
		f.formatCompact(buf, e)
	} else {
		f.formatStandard(buf, e)
	}

	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func (f *TextFormatter) formatStandard(buf *bytes.Buffer, e *Entry) {
	// LEVEL
	f.writeLevel(buf, e.Level)
	buf.WriteString(" | ")

	// TIME
	writeTimestamp(buf, e.Time)
	buf.WriteString(" | ")

	// GROUP
	writeGroups(buf, e.Logger.module, GetGroupKey(e.Data))
	buf.WriteString(" | ")

	// SOURCE
	if e.Logger.WithSource {
		buf.WriteString(e.Source)
		buf.WriteString(" | ")
	}

	// CtxFields
	if len(e.Data) > 0 {
		writeContextFields(buf, e.Data, ':')
		buf.WriteString(" | ")
	}

	// MESSAGE
	e.Message = strings.TrimSuffix(e.Message, "\n")
	buf.WriteString(e.Message)

	f.addFields(buf, e.Fields)
}

// addFields 将结构化字段追加到日志输出中。
func (f *TextFormatter) addFields(buf *bytes.Buffer, fields []Field) {
	if len(fields) == 0 {
		return
	}

	buf.WriteString(" ")

	for _, field := range fields {
		field.AddTo(buf)
	}
}

func (f *TextFormatter) formatCompact(buf *bytes.Buffer, e *Entry) {
	// LEVEL
	f.writeLevel(buf, e.Level)
	buf.WriteString(" ")

	// TIME
	writeTimestamp(buf, e.Time)
	buf.WriteString(" ")

	// GROUP
	buf.WriteString("[")
	writeGroups(buf, e.Logger.module, GetGroupKey(e.Data))
	buf.WriteString("]")
	buf.WriteString(" ")

	// SOURCE
	if e.Logger.WithSource {
		buf.WriteString(filepath.Base(e.Source))
		buf.WriteString(" ")
	}

	// FIELDS
	if len(e.Data) > 0 {
		writeContextFields(buf, e.Data, '=')
		buf.WriteString(" ")
	}

	// MESSAGE
	e.Message = strings.TrimSuffix(e.Message, "\n")
	buf.WriteString(e.Message)

	f.addFields(buf, e.Fields)
}

func writeTimestamp(buf *bytes.Buffer, t time.Time) {
	var scratch [32]byte
	buf.Write(t.AppendFormat(scratch[:0], timestampFormat))
}

func writeGroups(buf *bytes.Buffer, module string, groups []string) {
	buf.WriteString(module)
	for _, group := range groups {
		buf.WriteByte('/')
		buf.WriteString(group)
	}
}

func writeContextFields(buf *bytes.Buffer, data CtxFields, separator byte) {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(' ')
		}

		buf.WriteString(k)
		buf.WriteByte(separator)
		writeValue(buf, data[k])
	}
}

func writeValue(buf *bytes.Buffer, value any) {
	switch v := value.(type) {
	case nil:
		buf.WriteString("<nil>")
	case string:
		buf.WriteString(v)
	case bool:
		writeBool(buf, v)
	case int:
		writeInt(buf, int64(v))
	case int64:
		writeInt(buf, v)
	case int32:
		writeInt(buf, int64(v))
	case int16:
		writeInt(buf, int64(v))
	case int8:
		writeInt(buf, int64(v))
	case uint:
		writeUint(buf, uint64(v))
	case uint64:
		writeUint(buf, v)
	case uint32:
		writeUint(buf, uint64(v))
	case uint16:
		writeUint(buf, uint64(v))
	case uint8:
		writeUint(buf, uint64(v))
	case uintptr:
		writeUint(buf, uint64(v))
	case float64:
		writeFloat(buf, v, 64)
	case float32:
		writeFloat(buf, float64(v), 32)
	default:
		_, _ = fmt.Fprint(buf, v)
	}
}

func writeBool(buf *bytes.Buffer, value bool) {
	var scratch [5]byte
	buf.Write(strconv.AppendBool(scratch[:0], value))
}

func writeInt(buf *bytes.Buffer, value int64) {
	var scratch [20]byte
	buf.Write(strconv.AppendInt(scratch[:0], value, 10))
}

func writeUint(buf *bytes.Buffer, value uint64) {
	var scratch [20]byte
	buf.Write(strconv.AppendUint(scratch[:0], value, 10))
}

func writeFloat(buf *bytes.Buffer, value float64, bitSize int) {
	var scratch [32]byte
	buf.Write(strconv.AppendFloat(scratch[:0], value, 'g', -1, bitSize))
}

func (f *TextFormatter) writeLevel(buf *bytes.Buffer, l level.Level) {
	levelText := l.String()

	if f.isColored() {
		buf.WriteString("\x1b[")
		writeInt(buf, int64(f.colorForLevel(l)))
		buf.WriteByte('m')
	}

	buf.WriteString(levelText)
	for padding := f.levelTextMaxLength - utf8.RuneCountInString(levelText); padding > 0; padding-- {
		buf.WriteByte(' ')
	}

	if f.isColored() {
		buf.WriteString("\x1b[0m")
	}
}

func (f *TextFormatter) colorForLevel(l level.Level) int {
	switch l {
	case level.LevelDebug:
		return 36 // Cyan
	case level.LevelInfo:
		return 32 // Green
	case level.LevelWarn:
		return 33 // Yellow
	case level.LevelError:
		return 31 // Red
	default:
		return 37 // White
	}
}
