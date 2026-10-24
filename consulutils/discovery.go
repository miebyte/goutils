package consulutils

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
)

// ErrNoAvailableService 表示查询成功，但没有符合条件的可用服务地址。
// 调用方可用 errors.Is 判断；未注册、全部不健康或筛选后为空都会返回此错误。
var ErrNoAvailableService = errors.New("no available consul service")

// DiscoveryConfig 配置服务发现；默认跨命名空间查询，并复用公共 Consul client。
type DiscoveryConfig struct {
	Namespace string      // 可选，精确匹配服务的 Meta.namespace；空值表示不限制。
	Client    *api.Client // 可选，与配置读取和服务注册共用同一个 client。
}

// Discover 按目标服务名查询健康地址，返回排序、去重后的 host:port 列表。
// 最多接受一份配置，不读取 POD_NAMESPACE；Namespace 仅筛选服务元数据。
// 每次调用查询一次，最多等待 3 秒，并遵守 ctx 中更短的截止时间；不缓存、不重试。
func Discover(ctx context.Context, serviceName string, configs ...DiscoveryConfig) ([]string, error) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" || strings.ContainsAny(serviceName, "/?# \t\r\n") {
		return nil, errors.New("consul service name must not be empty or contain URL separators or spaces")
	}
	if len(configs) > 1 {
		return nil, errors.New("consul discovery accepts at most one configuration")
	}
	var config DiscoveryConfig
	if len(configs) == 1 {
		config = configs[0]
	}
	config.Namespace = strings.TrimSpace(config.Namespace)

	client := config.Client
	if client == nil {
		var err error
		client, err = DefaultClient()
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
		// 这里是应用注册的 Kubernetes 命名空间，与 Consul 自身的 Namespace 无关。
		if config.Namespace != "" && service.Meta["namespace"] != config.Namespace {
			continue
		}
		if service.Port < 1 || service.Port > 65535 || validateRegistrationHost(service.Address) != nil {
			continue
		}
		// 只使用业务注册地址；Node 地址和健康检查地址可能指向 Consul Agent 或管理端口。
		addresses = append(addresses, net.JoinHostPort(service.Address, strconv.Itoa(service.Port)))
	}
	if len(addresses) == 0 {
		err := fmt.Errorf("%w: service=%q namespace=%q", ErrNoAvailableService, serviceName, config.Namespace)
		if meta != nil && meta.ResultsFilteredByACLs {
			return nil, fmt.Errorf("%w (results filtered by ACLs)", err)
		}
		return nil, err
	}
	// 多个健康 Pod 可以注册相同的 Service 域名，避免把实例数量当成地址权重。
	slices.Sort(addresses)
	return slices.Compact(addresses), nil
}

// Resolve 从去重后的健康地址中随机选择一个，便于创建 HTTP 或 gRPC 客户端。
// 返回的是当前查询快照，不会自动更新调用方已经创建的连接。
func Resolve(ctx context.Context, serviceName string, configs ...DiscoveryConfig) (string, error) {
	addresses, err := Discover(ctx, serviceName, configs...)
	if err != nil {
		return "", err
	}
	return addresses[rand.IntN(len(addresses))], nil
}
