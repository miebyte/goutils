package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestNewFileWriterWritesLocalFile 验证日志会写入指定的本地文件。
func TestNewFileWriterWritesLocalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "app.log")
	w, err := NewFileWriter(path, WithMaxSize(10), WithMaxAge(7), WithMaxBackups(3), WithCompress(true))
	if err != nil {
		t.Fatalf("new file writer: %v", err)
	}
	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Errorf("close file writer: %v", err)
		}
	})

	if _, err := w.Write([]byte("hello file\n")); err != nil {
		t.Fatalf("write log: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if string(data) != "hello file\n" {
		t.Fatalf("log content = %q", data)
	}
}

// TestNewFileWriterRotatesBySize 验证超过大小上限后会留下备份文件。
func TestNewFileWriterRotatesBySize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	w, err := NewFileWriter(path, WithMaxSize(1), WithMaxBackups(1))
	if err != nil {
		t.Fatalf("new file writer: %v", err)
	}
	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Errorf("close file writer: %v", err)
		}
	})

	payload := bytes.Repeat([]byte("a"), 1024*1024)
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("write first chunk: %v", err)
	}
	if _, err := w.Write([]byte("b")); err != nil {
		t.Fatalf("write second chunk: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read log dir: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected rotated backup, got %d files", len(entries))
	}
}

// TestNewFileWriterRejectsEmptyFilename 验证空路径不会创建 Writer。
func TestNewFileWriterRejectsEmptyFilename(t *testing.T) {
	if _, err := NewFileWriter("  "); err == nil {
		t.Fatal("expected empty filename to fail")
	}
}
