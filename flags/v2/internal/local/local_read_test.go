package local

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/miebyte/goutils/flags/v2/provider"
)

func TestReadConfigMissingKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"port":3306}`), 0600); err != nil {
		t.Fatal(err)
	}
	local := NewLocalProvider(path)
	raw, err := local.ReadConfig(context.Background(), "port")
	if err != nil || string(raw) != "3306" {
		t.Fatalf("ReadConfig = %s, %v", raw, err)
	}
	if _, err := local.ReadConfig(context.Background(), "missing"); !errors.Is(err, provider.ErrConfigNotFound) {
		t.Fatalf("missing key error = %v", err)
	}
}

func TestReadConfigMissingFile(t *testing.T) {
	local := NewLocalProvider(filepath.Join(t.TempDir(), "missing.json"))
	if _, err := local.ReadConfig(context.Background(), "port"); !errors.Is(err, provider.ErrConfigNotFound) {
		t.Fatalf("missing file error = %v", err)
	}
}

func TestReadConfigCanceledContext(t *testing.T) {
	local := NewLocalProvider("")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := local.ReadConfig(ctx, "port"); !errors.Is(err, context.Canceled) || errors.Is(err, provider.ErrConfigNotFound) {
		t.Fatalf("canceled read error = %v", err)
	}
}

func TestInitializeRejectsInvalidConfig(t *testing.T) {
	for _, contents := range []string{"", "null", "[]", `{"port":`} {
		t.Run(contents, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			local := &LocalProvider{filePath: path}
			if err := local.initialize(); err == nil || errors.Is(err, provider.ErrConfigNotFound) {
				t.Fatalf("invalid config error = %v", err)
			}
		})
	}
}
