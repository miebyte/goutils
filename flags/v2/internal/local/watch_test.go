package local

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/miebyte/goutils/flags/v2/provider"
)

func TestWatchConfigPublishesKeyAndKeepsStartupCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	writeLocalConfig(t, path, `{"app":{"limit":1},"other":2}`)
	local := NewLocalProvider(path)
	t.Cleanup(func() { _ = local.Close() })
	events, err := local.WatchConfig("app")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		t.Fatalf("watch published an initial event: %+v", event)
	default:
	}

	writeLocalConfig(t, path, `{"app":{"limit":3},"other":4}`)
	waitLocalConfig(t, events, `{"limit":3}`)
	initial, err := local.ReadConfig(context.Background(), "app")
	if err != nil || string(initial) != `{"limit":1}` {
		t.Fatalf("startup cache = %s, %v", initial, err)
	}
}

func TestWatchConfigRelativePathAndAtomicReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	writeLocalConfig(t, path, `{"app":1}`)
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relativePath, err := filepath.Rel(workingDir, path)
	if err != nil {
		t.Fatal(err)
	}
	local := NewLocalProvider(relativePath)
	t.Cleanup(func() { _ = local.Close() })
	events, err := local.WatchConfig("app")
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(filepath.Dir(path), "replacement.json")
	writeLocalConfig(t, replacement, `{"app":2}`)
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	waitLocalConfig(t, events, "2")
	writeLocalConfig(t, path, `{"app":3}`)
	waitLocalConfig(t, events, "3")
}

func TestWatchConfigMissingInvalidAndRecreatedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	local := NewLocalProvider(path)
	t.Cleanup(func() { _ = local.Close() })
	events, err := local.WatchConfig("app")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		t.Fatalf("watch published an initial event for a missing file: %+v", event)
	default:
	}
	writeLocalConfig(t, path, `{"other":1}`)
	for {
		if event := nextLocalEvent(t, events); errors.Is(event.Err, provider.ErrConfigNotFound) {
			break
		}
	}
	writeLocalConfig(t, path, `{"app":2}`)
	waitLocalConfig(t, events, "2")

	writeLocalConfig(t, path, `invalid`)
	for {
		event := nextLocalEvent(t, events)
		if event.Err != nil && !errors.Is(event.Err, provider.ErrConfigNotFound) {
			break
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	for {
		if event := nextLocalEvent(t, events); errors.Is(event.Err, provider.ErrConfigNotFound) {
			break
		}
	}
	writeLocalConfig(t, path, `{"app":3}`)
	waitLocalConfig(t, events, "3")
}

func TestWatchConfigCloseWithoutConsumers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	writeLocalConfig(t, path, `{"app":1,"other":2}`)
	local := NewLocalProvider(path)
	t.Cleanup(func() { _ = local.Close() })
	var streams []<-chan provider.Event
	for _, key := range []string{"app", "other"} {
		events, err := local.WatchConfig(key)
		if err != nil {
			t.Fatal(err)
		}
		streams = append(streams, events)
	}
	writeLocalConfig(t, path, `{"app":3,"other":4}`)
	closed := make(chan struct{})
	go func() {
		_ = local.Close()
		_ = local.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("Close blocked with unread events")
	}
	for _, events := range streams {
		for range events {
		}
	}
	if events, err := local.WatchConfig("app"); events != nil || !errors.Is(err, os.ErrClosed) {
		t.Fatalf("watch after Close = %v, %v", events, err)
	}
}

func writeLocalConfig(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

func nextLocalEvent(t *testing.T, events <-chan provider.Event) provider.Event {
	t.Helper()
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("watch closed before expected event")
		}
		return event
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for local config event")
		return provider.Event{}
	}
}

func waitLocalConfig(t *testing.T, events <-chan provider.Event, want string) {
	t.Helper()
	for {
		if event := nextLocalEvent(t, events); event.Err == nil && string(event.Config) == want {
			return
		}
	}
}
