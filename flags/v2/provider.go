package flags

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/miebyte/goutils/flags/v2/internal/config"
	"github.com/miebyte/goutils/flags/v2/internal/consul"
	"github.com/miebyte/goutils/flags/v2/internal/local"
	"github.com/miebyte/goutils/flags/v2/provider"
)

type ConfigProvider = provider.ConfigProvider
type ConfigWatcher = provider.ConfigWatcher
type Event = provider.Event

// NewConsulProvider 延迟创建 Consul 客户端。config 为 nil 时使用 SDK 默认配置及 CONSUL_* 环境变量。
// 默认 KV 路径为 {prefix}/{key}；prefix 为空时直接使用 key。
func NewConsulProvider(config *consulapi.Config, prefix string) ConfigProvider {
	return &remoteProvider{
		newClient: func() (ConfigProvider, error) { return consul.NewConsulClient(config, prefix) },
	}
}

// WithRemoteConsulProvider 覆盖 WithRemoteConfig 使用的 Consul 连接和 KV key 前缀。
func WithRemoteConsulProvider(config *consulapi.Config, prefix string) RemoteConfigOptionFunc {
	return func(o *RemoteConfigOption) {
		o.provider = NewConsulProvider(config, prefix)
	}
}

// remoteProvider 给 ConfigProvider 包装一层，实现延时初始化
type remoteProvider struct {
	mu        sync.Mutex
	closed    bool
	client    ConfigProvider
	closeErr  error
	newClient func() (ConfigProvider, error)
}

func (p *remoteProvider) getClient(ctx context.Context) (ConfigProvider, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, fmt.Errorf("config provider is closed")
	}
	if p.client == nil {
		client, err := p.newClient()
		if err != nil {
			return nil, err
		}
		p.client = client
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p.client, nil
}

func (p *remoteProvider) ReadConfig(ctx context.Context, path string) (json.RawMessage, error) {
	client, err := p.getClient(ctx)
	if err != nil {
		return nil, err
	}
	return client.ReadConfig(ctx, path)
}

func (p *remoteProvider) WatchConfig(path string) (<-chan Event, error) {
	client, err := p.getClient(context.Background())
	if err != nil {
		return nil, err
	}
	return client.WatchConfig(path)
}

func (p *remoteProvider) Path(key string) string {
	client, err := p.getClient(context.Background())
	if err != nil {
		return ""
	}
	return client.Path(key)
}

func (p *remoteProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		p.closed = true
		if p.client != nil {
			p.closeErr = p.client.Close()
		}
	}
	return p.closeErr
}

func configPath(source ConfigProvider, key, path string) (string, error) {
	if path != "" {
		return path, nil
	}

	return source.Path(key), nil
}

func getConfigProvider(opt *Option) (ConfigProvider, error) {
	if opt.ConfigPath != "" || !opt.UseRemote {
		path := config.LocalConfigPath(opt.ConfigPath)
		if path == "" && opt.WatchConfig {
			path = "config.json"
		}
		return local.NewLocalProvider(path), nil
	}

	if opt.RemoteOption.provider != nil {
		return opt.RemoteOption.provider, nil
	}

	return NewConsulProvider(nil, ""), nil
}
