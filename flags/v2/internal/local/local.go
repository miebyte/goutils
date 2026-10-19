package local

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/miebyte/goutils/flags/v2/provider"
	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/utils/syncx"
	"github.com/pkg/errors"
)

type LocalProvider struct {
	filePath string
	// config 用于存储本地配置内容
	// 后续 ReadConfig 仅会从此处进行读取
	config *syncx.SyncMapX[string, json.RawMessage]
	mu     sync.Mutex
	done   chan struct{}
	wg     sync.WaitGroup
}

func NewLocalProvider(filePath string) *LocalProvider {
	if filePath != "" {
		var err error
		filePath, err = filepath.Abs(filePath)
		innerlog.Logger.PanicError(err)
	}
	lr := &LocalProvider{
		filePath: filePath,
		config:   syncx.NewSyncMapX[string, json.RawMessage](),
		done:     make(chan struct{}),
	}

	// 优先初始化，尝试读取本地配置
	err := lr.initialize()
	innerlog.Logger.PanicError(err)

	return lr
}

func (l *LocalProvider) initialize() error {
	b, err := l.readFile()
	if err != nil || b == nil {
		return err
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	if fields == nil {
		return errors.New("local config must be a JSON object")
	}

	for k, v := range fields {
		l.config.Store(k, v)
	}

	return nil
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

func (l *LocalProvider) readFile() ([]byte, error) {
	if !l.fileExists(l.filePath) {
		return nil, nil
	}

	b, err := os.ReadFile(l.filePath)
	if err != nil {
		return nil, errors.Wrap(err, "readFile")
	}

	innerlog.Logger.Infof("Read local config success. Config=%s", l.filePath)
	return b, nil
}

func (l *LocalProvider) ReadConfig(ctx context.Context, key string) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	config, ok := l.config.Load(key)
	if !ok {
		return nil, errors.Wrapf(provider.ErrConfigNotFound, "local config %q key %q", l.filePath, key)
	}

	return config, nil
}

func (l *LocalProvider) WatchConfig(key string) (<-chan provider.Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	select {
	case <-l.done:
		return nil, os.ErrClosed
	default:
	}
	if l.filePath == "" {
		innerlog.Logger.Debugf("no local config file set, skip watch")
		return nil, errors.New("no local config file set")
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		innerlog.Logger.Errorf("create watcher error: %v", err)
		return nil, errors.Wrap(err, "create watcher")
	}

	dir := filepath.Dir(l.filePath)
	if err := w.Add(dir); err != nil {
		innerlog.Logger.Errorf("watch dir error: %v", err)
		_ = w.Close()
		return nil, errors.Wrap(err, "add watcher")
	}

	ch := make(chan provider.Event, 1)
	l.wg.Add(1)

	go func() {
		defer l.wg.Done()
		defer w.Close()
		defer close(ch)
		for {
			var event provider.Event
			select {
			case <-l.done:
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if ev.Name != l.filePath || ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
					continue
				}
				// kqueue 可能在 Create 事件之后才订阅新文件，读取前先补上监听。
				if ev.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
					_ = w.Add(l.filePath)
				}
				event = l.watchEvent(key)
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				event = provider.Event{Path: l.filePath, Err: err}
			}
			select {
			case ch <- event:
			case <-l.done:
				return
			}
		}
	}()
	return ch, nil
}

func (l *LocalProvider) watchEvent(key string) provider.Event {
	event := provider.Event{Path: l.filePath}
	b, err := l.readFile()
	if err != nil {
		event.Err = err
		return event
	}
	if b == nil {
		event.Err = provider.ErrConfigNotFound
		return event
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		event.Err = err
	} else if fields == nil {
		event.Err = errors.New("local config must be a JSON object")
	} else if value, ok := fields[key]; ok {
		event.Config = value
	} else {
		event.Err = provider.ErrConfigNotFound
	}
	return event
}

func (l *LocalProvider) Path(key string) string {
	return l.filePath
}

func (l *LocalProvider) Close() error {
	l.mu.Lock()
	select {
	case <-l.done:
	default:
		close(l.done)
	}
	l.mu.Unlock()
	l.wg.Wait()
	return nil
}
