package flags

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/miebyte/goutils/flags/v2/provider"
	"github.com/miebyte/goutils/internal/innerlog"
)

type configWatch struct {
	source  ConfigProvider
	path    string
	readers []keyReader
}

// WatchConfig 监听已声明的 Struct，同一来源和路径只订阅一次。
// 应在 ReadConfig 后调用，重复调用不会重复订阅。
func (sf *SuperFlags) WatchConfig() error {
	sf.watchOnce.Do(func() {
		sf.watchErr = sf.watchConfig()
		if sf.watchErr != nil {
			_ = sf.Close()
		}
	})
	return sf.watchErr
}

func (sf *SuperFlags) watchConfig() error {
	sf.mu.RLock()
	source := sf.defaultConfigProvider

	// 获取所有需要监听的配置
	// 只要 reader 中 confRefresh 不为 nil，就认为需要监听。
	readers := make([]keyReader, 0, len(sf.structKeyReaders))
	for _, reader := range sf.structKeyReaders {
		if reader.confRefresh != nil {
			readers = append(readers, reader)
		}
	}
	sf.mu.RUnlock()
	if source == nil {
		return nil
	}

	// 将需要监听的配置分组，每个组由一个 ConfigProvider 和路径组成。
	// 因为 Struct 可以指定不同的 ConfigProvider，所以需要分组。
	var watches []configWatch
	for _, reader := range readers {
		// 解析当前 reader 的配置源和路径。
		p, path, err := reader.resolve(source)
		if err != nil {
			return fmt.Errorf("resolve config %q: %w", reader.key, err)
		}

		// 检查是否已经存在相同的组，如果存在则将当前 reader 添加到组中，否则创建一个新组。
		matched := false
		for i := range watches {
			if sameProvider(watches[i].source, p) && watches[i].path == path {
				watches[i].readers = append(watches[i].readers, reader)
				matched = true
				break
			}
		}
		if !matched {
			watches = append(watches, configWatch{p, path, []keyReader{reader}})
		}
	}

	// 遍历所有需要监听的配置，并创建一个 Watcher 来监听配置变化。
	for _, watch := range watches {
		events, err := watch.source.WatchConfig(watch.path)
		if err != nil {
			return fmt.Errorf("watch config %q: %w", watch.path, err)
		}
		if events == nil {
			return fmt.Errorf("watch config %q returned a nil channel", watch.path)
		}

		sf.watchers.Go(func() {
			for event := range events {
				// watch 事件返回了一个错误
				if event.Err != nil {
					if !errors.Is(event.Err, provider.ErrConfigNotFound) {
						innerlog.Logger.Errorf("watch config %q: %v", watch.path, event.Err)
					}
					continue
				}

				// watch 没有返回配置
				if len(event.Config) == 0 {
					innerlog.Logger.Errorf("watch config %q returned an empty config", watch.path)
					continue
				}

				for _, reader := range watch.readers {
					if err := reader.confRefresh(event.Config); err != nil {
						innerlog.Logger.Errorf("refresh config %q: %v", reader.key, err)
						continue
					}
					innerlog.Logger.Infof("refresh config %q success", reader.key)
				}
			}
		})
	}
	return nil
}

func sameProvider(a, b ConfigProvider) bool {
	return reflect.TypeOf(a) == reflect.TypeOf(b) && reflect.ValueOf(a).Comparable() && a == b
}

// Close 关闭配置来源，并等待已接收的配置更新处理完毕。
// 应在初始化完成后、服务退出时调用。
func (sf *SuperFlags) Close() error {
	sf.closeOnce.Do(func() {
		sf.mu.RLock()
		sources := []ConfigProvider{sf.defaultConfigProvider}
		if !sf.isLocal {
			for _, reader := range sf.structKeyReaders {
				sources = append(sources, reader.provider)
			}
		}
		sf.mu.RUnlock()
		for _, source := range sources {
			if source != nil {
				sf.closeErr = errors.Join(sf.closeErr, source.Close())
			}
		}
		sf.watchers.Wait()
	})
	return sf.closeErr
}

// Close 释放 Parse 使用的配置连接和监听。
func Close() error { return sf.Close() }
