package discover

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/miebyte/goutils/consulutils"
)

// Consul 通过 Consul Health API 发现健康实例，零值复用进程共享的 client。
type Consul struct {
	Client *api.Client
}

func NewConsulDiscovery(client *api.Client) *Consul {
	return &Consul{
		Client: client,
	}
}

func (c *Consul) Discover(ctx context.Context, serviceName string) ([]string, error) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" || strings.ContainsAny(serviceName, "/?# \t\r\n") {
		return nil, errors.New("consul service name must not be empty or contain URL separators or spaces")
	}

	client := c.Client
	if client == nil {
		var err error
		client, err = consulutils.DefaultClient()
		if err != nil {
			return nil, err
		}
	}

	requestCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// passing=true 由 Consul 筛选节点和服务检查均为 passing 的实例。
	entries, meta, err := client.Health().Service(serviceName, "", true, (&api.QueryOptions{}).WithContext(requestCtx))
	if err != nil {
		return nil, fmt.Errorf("discover consul service %q: %w", serviceName, err)
	}

	addresses := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry == nil || entry.Service == nil {
			continue
		}
		service := entry.Service
		if service.Port < 1 || service.Port > 65535 || validateRegistrationHost(service.Address) != nil {
			continue
		}
		// 只使用业务注册地址；Node 地址和健康检查地址可能指向 Consul Agent 或管理端口。
		addresses = append(addresses, net.JoinHostPort(service.Address, strconv.Itoa(service.Port)))
	}
	if len(addresses) == 0 {
		err := fmt.Errorf("%w: service=%q", ErrNoAvailableService, serviceName)
		if meta != nil && meta.ResultsFilteredByACLs {
			return nil, fmt.Errorf("%w (results filtered by ACLs)", err)
		}
		return nil, err
	}
	// 多个健康 Pod 可以注册相同的 Service 域名，避免把实例数量当成地址权重。
	slices.Sort(addresses)
	return slices.Compact(addresses), nil
}
