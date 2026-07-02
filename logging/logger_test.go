package logging

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestPrettyLoggerInfowWritesStructuredFields 验证结构化字段与上下文字段会分别输出。
func TestPrettyLoggerInfowWritesStructuredFields(t *testing.T) {
	var buf bytes.Buffer

	logger := NewPrettyLogger(&buf, WithModule("TEST"), WithEnableSource(false))
	ctx := With(context.Background(), "request_id", "req-1")

	logger.Infow(ctx, "hello", String("user", "alice"), Int("age", 18))

	output := buf.String()
	if !strings.Contains(output, "request_id:req-1") {
		t.Fatalf("expected ctx fields in output, got %q", output)
	}

	if !strings.Contains(output, "hello user=alice age=18") {
		t.Fatalf("expected structured fields in output, got %q", output)
	}
}
