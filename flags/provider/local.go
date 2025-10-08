package provider

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/pkg/errors"
)

type LocalProvider struct {
	filePath string
}

func NewLocalProvider(filePath string) *LocalProvider {
	return &LocalProvider{filePath: filePath}
}

func (l *LocalProvider) fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func (l *LocalProvider) ReadConfig() (map[string]any, error) {
	if !l.fileExists(l.filePath) {
		return nil, nil
	}

	b, err := os.ReadFile(l.filePath)
	if err != nil {
		return nil, errors.Wrap(err, "readFile")
	}

	temp := make(map[string]any)
	if err := json.Unmarshal(b, &temp); err != nil {
		return nil, errors.Wrap(err, "unmarshalLocalConfig")
	}

	innerlog.Logger.Infof("Read local config success. Config=%s", l.filePath)
	// normalize keys to lower-case, recursively
	return normalizeMapCaseInsensitive(temp), nil
}

func (l *LocalProvider) WatchConfig() <-chan Event {
	if l.filePath == "" {
		innerlog.Logger.Debugf("no local config file set, skip watch")
		ch := make(chan Event)
		close(ch)
		return ch
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		innerlog.Logger.Errorf("create watcher error: %v", err)
		ch := make(chan Event)
		close(ch)
		return ch
	}
	ch := make(chan Event)

	dir := filepath.Dir(l.filePath)
	if err := w.Add(dir); err != nil {
		innerlog.Logger.Errorf("watch dir error: %v", err)
		close(ch)
		_ = w.Close()
		return ch
	}

	go func() {
		defer w.Close()
		for {
			select {
			case ev := <-w.Events:
				if ev.Name == l.filePath && (ev.Op&fsnotify.Write == fsnotify.Write || ev.Op&fsnotify.Create == fsnotify.Create) {
					innerlog.Logger.Debugf("local config change: %s", ev.Name)
					event := Event{Path: l.filePath}
					event.Config, event.Err = l.ReadConfig()
					ch <- event
				}
			case err := <-w.Errors:
				innerlog.Logger.Errorf("watch error: %v", err)
				close(ch)
				return
			}
		}
	}()
	return ch
}
