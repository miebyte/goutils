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
	"unicode/utf8"

	"github.com/mattn/go-isatty"
	"github.com/miebyte/goutils/logging/level"
	"github.com/miebyte/goutils/masking"
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
	buf.WriteString(f.formatLevel(e.Level))
	buf.WriteString(" | ")

	// TIME
	buf.WriteString(e.Time.Format("2006-01-02 15:04:05.000"))
	buf.WriteString(" | ")

	// GROUP
	groups := append([]string{e.Logger.module}, GetGroupKey(e.Data)...)
	if len(groups) > 0 {
		buf.WriteString(strings.Join(groups, "/"))
	} else {
		buf.WriteString("DEFAULT")
	}
	buf.WriteString(" | ")

	// SOURCE
	var source string
	if e.Caller != nil {
		pkg := getPackageName(e.Caller.Function)
		file := filepath.Base(e.Caller.File)
		if pkg != "" {
			source = pkg + "/" + file
		} else {
			source = file
		}
		source = fmt.Sprintf("%s:%d", source, e.Caller.Line)
		buf.WriteString(source)
		buf.WriteString(" | ")
	}

	// FIELDS
	if len(e.Data) > 0 {
		keys := make([]string, 0, len(e.Data))
		for k := range e.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		fieldParts := make([]string, 0, len(keys))
		for _, k := range keys {
			fieldParts = append(fieldParts, fmt.Sprintf("%s:%v", k, e.Data[k]))
		}
		buf.WriteString(strings.Join(fieldParts, " "))
		buf.WriteString(" | ")
	}

	// MESSAGE
	e.Message = strings.TrimSuffix(e.Message, "\n")
	e.Message = masking.MaskMessage(e.Message)
	buf.WriteString(e.Message)
}

func (f *TextFormatter) formatCompact(buf *bytes.Buffer, e *Entry) {
	// LEVEL
	buf.WriteString(f.formatLevel(e.Level))
	buf.WriteString(" ")

	// TIME
	buf.WriteString(e.Time.Format("2006-01-02 15:04:05.000"))
	buf.WriteString(" ")

	// GROUP
	groups := append([]string{e.Logger.module}, GetGroupKey(e.Data)...)
	if len(groups) > 0 {
		buf.WriteString("[")
		buf.WriteString(strings.Join(groups, "/"))
		buf.WriteString("]")
		buf.WriteString(" ")
	}

	// SOURCE
	if e.Caller != nil {
		file := filepath.Base(e.Caller.File)
		buf.WriteString(file)
		buf.WriteString(":")
		buf.WriteString(strconv.Itoa(e.Caller.Line))
		buf.WriteString(" ")
	}

	// FIELDS
	if len(e.Data) > 0 {
		keys := make([]string, 0, len(e.Data))
		for k := range e.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		fieldParts := make([]string, 0, len(keys))
		for _, k := range keys {
			fieldParts = append(fieldParts, fmt.Sprintf("%s=%v", k, e.Data[k]))
		}
		buf.WriteString(strings.Join(fieldParts, " "))
		buf.WriteString(" ")
	}

	// MESSAGE
	e.Message = strings.TrimSuffix(e.Message, "\n")
	e.Message = masking.MaskMessage(e.Message)
	buf.WriteString(e.Message)
}

func (f *TextFormatter) formatLevel(l level.Level) string {
	levelText := l.String()
	formatString := "%-" + strconv.Itoa(f.levelTextMaxLength) + "s"
	levelText = fmt.Sprintf(formatString, levelText)

	if f.isColored() {
		return fmt.Sprintf("\x1b[%dm%s\x1b[0m", f.colorForLevel(l), levelText)
	}
	return levelText
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
