package consul

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	consulwatch "github.com/hashicorp/consul/api/watch"
	"github.com/miebyte/goutils/flags/v2/provider"
)

type ConsulProvider struct {
	client     *consulapi.Client
	prefix     string
	httpClient *http.Client
	done       chan struct{}
	doneOnce   sync.Once
	closeOnce  sync.Once
	mu         sync.Mutex
	plans      []*consulwatch.Plan
	watchers   sync.WaitGroup
}

var _ provider.ConfigProvider = (*ConsulProvider)(nil)

func NewConsulClient(config *consulapi.Config, prefix string) (*ConsulProvider, error) {
	if config == nil {
		config = consulapi.DefaultConfig()
	}

	// SDK 会补全配置，使用副本避免修改调用方的配置。
	cfg := *config
	ownClient := cfg.HttpClient == nil
	if ownClient && cfg.Transport != nil {
		cfg.Transport = cfg.Transport.Clone()
	}
	client, err := consulapi.NewClient(&cfg)
	if err != nil {
		return nil, fmt.Errorf("create consul client: %w", err)
	}

	p := &ConsulProvider{
		client: client,
		prefix: strings.Trim(prefix, "/"),
	}

	if ownClient {
		p.httpClient = cfg.HttpClient
	}
	return p, nil
}

func (p *ConsulProvider) ReadConfig(ctx context.Context, path string) (json.RawMessage, error) {
	pair, _, err := p.client.KV().Get(path, (&consulapi.QueryOptions{}).WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("read consul config %q: %w", path, err)
	}

	if pair == nil {
		return nil, fmt.Errorf("consul config %q: %w", path, provider.ErrConfigNotFound)
	}
	return pair.Value, nil
}

func (p *ConsulProvider) WatchConfig(path string) (<-chan provider.Event, error) {
	// 创建一个 Watch 计划
	plan, err := consulwatch.Parse(map[string]any{"type": "key", "key": path})
	if err != nil {
		return nil, err
	}

	events := make(chan provider.Event, 1)
	ready := make(chan struct{})

	var previous json.RawMessage
	var previousExists, initialized bool
	plan.Handler = func(_ uint64, data any) {
		pair, _ := data.(*consulapi.KVPair)
		exists := pair != nil
		var raw json.RawMessage
		if pair != nil {
			raw = pair.Value
		}

		// 官方 Watch 会回调初值；provider 仅发布其后的内容变更。
		if !initialized {
			previous, previousExists, initialized = raw, exists, true
			close(ready)
			return
		}
		if previousExists == exists && bytes.Equal(previous, raw) {
			return
		}

		// SDK 保存 KVPair 用于后续比较，事件使用独立副本。
		event := provider.Event{Path: path, Config: bytes.Clone(raw)}
		if !exists {
			event.Err = provider.ErrConfigNotFound
		}

		if !p.sendLatest(events, event) {
			return
		}
		previous, previousExists = raw, exists
	}

	// 登记监听和关闭共用一把锁，确保 Close 开始后不会再添加任务。
	p.mu.Lock()
	select {
	case <-p.doneSignal():
		p.mu.Unlock()
		return nil, os.ErrClosed
	default:
	}
	p.plans = append(p.plans, plan)
	p.watchers.Go(func() {
		defer close(events)

		err := plan.RunWithClientAndHclog(p.client, nil)
		if err != nil {
			p.sendLatest(events, provider.Event{Path: path, Err: err})
		}
	})
	p.mu.Unlock()

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-ready:
		return events, nil
	case <-p.doneSignal():
		return nil, os.ErrClosed
	case <-timer.C:
		plan.Stop()
		return nil, fmt.Errorf("initialize consul watch %q: %w", path, context.DeadlineExceeded)
	}
}

func (p *ConsulProvider) sendLatest(events chan provider.Event, event provider.Event) bool {
	select {
	case <-p.doneSignal():
		return false
	default:
	}
	select {
	case events <- event:
		return true
	default:
	}

	// 如果事件通道已满，则丢弃最早的事件
	select {
	case <-events:
	default:
	}

	// 重新发送最新的事件
	events <- event
	return true
}

func (p *ConsulProvider) Path(key string) string {
	key = strings.TrimLeft(key, "/")
	if p.prefix == "" {
		return key
	}
	return p.prefix + "/" + key
}

func (p *ConsulProvider) doneSignal() chan struct{} {
	p.doneOnce.Do(func() { p.done = make(chan struct{}) })
	return p.done
}

func (p *ConsulProvider) Close() error {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		close(p.doneSignal())
		plans := p.plans
		p.plans = nil
		p.mu.Unlock()

		for _, plan := range plans {
			plan.Stop()
		}
		p.watchers.Wait()
		if p.httpClient != nil {
			p.httpClient.CloseIdleConnections()
		}
	})
	return nil
}
