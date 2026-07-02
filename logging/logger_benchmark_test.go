package logging

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// newBenchmarkLogger 创建基准测试使用的日志实例。
func newBenchmarkLogger() *PrettyLogger {
	logger := NewPrettyLogger(io.Discard, WithModule("BENCH"), WithEnableSource(false))
	logger.SetNoLock()
	return logger
}

// newBenchmarkContext 创建带上下文字段的基准测试上下文。
func newBenchmarkContext() context.Context {
	ctx := context.Background()
	ctx = With(ctx, "request_id", "req-1")
	ctx = With(ctx, "user", "alice")
	ctx = With(ctx, "attempt", 3)
	return ctx
}

// newBenchmarkFields 创建基准测试使用的结构化字段切片。
func newBenchmarkFields() []Field {
	return []Field{
		String("request_id", "req-1"),
		String("user", "alice"),
		Int("attempt", 3),
	}
}

// BenchmarkPrettyLoggerContextVsField 对比 ContextLogger 与 FieldLogger 的性能差异。
func BenchmarkPrettyLoggerContextVsField(b *testing.B) {
	logger := newBenchmarkLogger()
	ctxWithFields := newBenchmarkContext()
	ctxWithoutFields := context.Background()
	fields := newBenchmarkFields()

	b.Run("context_logger", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			logger.Infoc(ctxWithFields, "hello")
		}
	})

	b.Run("field_logger", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			logger.Infow(ctxWithoutFields, "hello", fields...)
		}
	})

	b.Run("sugared_logger", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			logger.Infos(ctxWithoutFields, "hello", "request_id", "req-1", "user", "alice", "attempt", 3)
		}
	})

	b.Run("origin", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		slogLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

		for i := 0; i < b.N; i++ {
			slogLogger.Info("hello", "request_id", "req-1", "user", "alice", "attempt", 3)
		}
	})
}
