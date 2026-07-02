package logging

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/miebyte/goutils/logging/level"
)

func TestTextFormatterOutputFormat(t *testing.T) {
	logger := NewPrettyLogger(&bytes.Buffer{}, WithModule("TEST"), WithEnableSource(false))
	logTime := time.Date(2026, 7, 2, 15, 4, 5, 123_000_000, time.UTC)

	tests := []struct {
		name     string
		compact  bool
		expected string
	}{
		{
			name:     "standard",
			compact:  false,
			expected: "INFO  | 2026-07-02 15:04:05.123 | TEST/api/v1 | attempt:3 request_id:req-1 z_float:1.2e+20 | hello user=alice age=18 \n",
		},
		{
			name:     "compact",
			compact:  true,
			expected: "INFO  2026-07-02 15:04:05.123 [TEST/api/v1] attempt=3 request_id=req-1 z_float=1.2e+20 hello user=alice age=18 \n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := &TextFormatter{CompactMode: tt.compact}
			entry := &Entry{
				Logger:  logger,
				Data:    newFormatterTestData(),
				Fields:  []Field{String("user", "alice"), Int("age", 18)},
				Time:    logTime,
				Level:   level.LevelInfo,
				Message: "hello\n",
			}

			serialized, err := formatter.Format(entry)
			if err != nil {
				t.Fatalf("format entry: %v", err)
			}

			if got := string(serialized); got != tt.expected {
				t.Fatalf("unexpected output\nwant: %q\n got: %q", tt.expected, got)
			}
		})
	}
}

func newFormatterTestData() CtxFields {
	return CtxFields{
		LoggingGroupKey: []string{"api", "v1"},
		"request_id":    "req-1",
		"attempt":       3,
		"z_float":       1.2e20,
	}
}

type mutatingHook struct{}

func (mutatingHook) Name() string {
	return "mutating"
}

func (mutatingHook) Levels() []level.Level {
	return []level.Level{level.LevelInfo}
}

func (mutatingHook) Fire(entry *Entry) error {
	delete(entry.Data, "request_id")
	entry.Data["hook"] = "value"
	return nil
}

func TestPrettyLoggerContextFieldsStayIsolated(t *testing.T) {
	var buf bytes.Buffer
	logger := NewPrettyLogger(&buf, WithModule("TEST"), WithEnableSource(false))
	logger.AddHook(mutatingHook{})

	ctx := With(context.Background(), "request_id", "req-1")
	logger.Infoc(ctx, "hello")

	fields := ctx.Value(FieldContextKey).(CtxFields)
	if got := fields["request_id"]; got != "req-1" {
		t.Fatalf("expected context request_id to remain unchanged, got %v", got)
	}
	if _, exists := fields["hook"]; exists {
		t.Fatalf("hook field leaked back into context fields: %#v", fields)
	}
}

func TestPrettyLoggerConcurrentSharedContext(t *testing.T) {
	var buf bytes.Buffer
	logger := NewPrettyLogger(&buf, WithModule("TEST"), WithEnableSource(false))
	ctx := With(context.Background(), "request_id", "req-1")

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Infoc(ctx, "hello")
		}()
	}
	wg.Wait()

	output := buf.String()
	if count := strings.Count(output, "request_id:req-1"); count != 32 {
		t.Fatalf("expected 32 log lines with request_id, got %d in %q", count, output)
	}
}
