package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalConfigPathSearchOrder(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	t.Chdir(cwd)
	t.Setenv("HOME", home)
	if err := os.Mkdir(filepath.Join(cwd, "configs"), 0700); err != nil {
		t.Fatal(err)
	}
	paths := []string{
		filepath.Join(cwd, "config.json"),
		filepath.Join(cwd, "configs", "config.json"),
		filepath.Join(home, "config.json"),
	}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range paths {
		if got := LocalConfigPath(""); got != want {
			t.Fatalf("LocalConfigPath = %q, want %q", got, want)
		}
		if err := os.Remove(want); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLocalConfigPathExplicitDoesNotFallBack(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HOME", "")
	if err := os.WriteFile("config.json", []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("missing", "explicit.json")
	if got := LocalConfigPath(want); got != want {
		t.Fatalf("LocalConfigPath = %q, want explicit path %q", got, want)
	}
}

func TestLocalConfigPathWithoutFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HOME", "")
	if got := LocalConfigPath(""); got != "" {
		t.Fatalf("LocalConfigPath = %q, want no path", got)
	}
}

func TestLocalConfigPathDoesNotCache(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	t.Setenv("HOME", "")
	if got := LocalConfigPath(""); got != "" {
		t.Fatalf("LocalConfigPath before creation = %q", got)
	}
	want := filepath.Join(cwd, "config.json")
	if err := os.WriteFile(want, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := LocalConfigPath(""); got != want {
		t.Fatalf("LocalConfigPath after creation = %q, want %q", got, want)
	}
	if err := os.Remove(want); err != nil {
		t.Fatal(err)
	}
	if got := LocalConfigPath(""); got != "" {
		t.Fatalf("LocalConfigPath after removal = %q", got)
	}
}
