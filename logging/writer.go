package logging

import (
	"errors"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// FileWriter 把日志写入本地文件，并按大小、保留数量和保留天数轮转。
// 轮转实现由 lumberjack 提供。备份文件名使用本地时间。
type FileWriter = lumberjack.Logger

// FileWriterOption 调整 lumberjack 的轮转参数。
type FileWriterOption func(*lumberjack.Logger)

// WithMaxSize 设置单个日志文件的大小上限，单位是 MB。
// 小于等于 0 时沿用 lumberjack 的默认值 100MB。
func WithMaxSize(megabytes int) FileWriterOption {
	return func(w *lumberjack.Logger) {
		if megabytes > 0 {
			w.MaxSize = megabytes
		}
	}
}

// WithMaxAge 设置备份文件的保留天数。小于等于 0 时不按时间删除备份。
func WithMaxAge(days int) FileWriterOption {
	return func(w *lumberjack.Logger) {
		if days > 0 {
			w.MaxAge = days
		}
	}
}

// WithMaxBackups 设置最多保留的备份数量。小于等于 0 时不按数量删除备份。
func WithMaxBackups(n int) FileWriterOption {
	return func(w *lumberjack.Logger) {
		if n > 0 {
			w.MaxBackups = n
		}
	}
}

// WithCompress 控制是否把备份压缩为 gzip。
func WithCompress(on bool) FileWriterOption {
	return func(w *lumberjack.Logger) {
		w.Compress = on
	}
}

// NewFileWriter 创建一个写入本地文件并自动轮转的 Writer。
func NewFileWriter(filename string, opts ...FileWriterOption) (*lumberjack.Logger, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, errors.New("log filename is empty")
	}

	w := &lumberjack.Logger{
		Filename:  filename,
		LocalTime: true,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(w)
		}
	}
	return w, nil
}
