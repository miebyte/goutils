// Package discover 统一按服务名发现地址，支持 Consul 和本地 JSON 服务目录。
package discover

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"slices"
	"strings"
	"sync/atomic"
)

var (
	// ErrNoAvailableService 表示查询成功，但服务不存在或没有可用地址。
	ErrNoAvailableService = errors.New("no available service")
	// ErrNoDataSource 表示没有可用的数据源。
	ErrNoDataSource = errors.New("no data source")
)

// Discoverer 按服务名查询 host:port 地址。实现应支持并发调用，并遵守 ctx 的取消和期限。
// 没有可用地址时应返回 ErrNoAvailableService；查询失败应保留原始错误。
type Discoverer interface {
	Discover(ctx context.Context, serviceName string) ([]string, error)
}

var defaultResolver atomic.Pointer[Discoverer]

func currentResolver() Discoverer {
	resolver := defaultResolver.Load()
	if resolver != nil {
		return *resolver
	}

	return nil
}

func SetDefaultResolver(resolver Discoverer) {
	if resolver == nil {
		defaultResolver.Store(nil)
		return
	}
	defaultResolver.Store(&resolver)
}

// SetDefaultResolverIfUnset 仅在尚未设置数据源时安装默认值，不覆盖显式配置。
// 与 SetDefaultResolver 并发调用时，显式设置始终优先。
func SetDefaultResolverIfUnset(resolver Discoverer) bool {
	if resolver == nil {
		return false
	}
	return defaultResolver.CompareAndSwap(nil, &resolver)
}

// Discover 返回排序、去重后的 host:port。未配置时默认查询 Consul。
func Discover(ctx context.Context, serviceName string) ([]string, error) {
	addresses, err := currentResolver().Discover(ctx, serviceName)
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("%w: service=%q", ErrNoAvailableService, serviceName)
	}
	addresses = slices.Clone(addresses)
	slices.Sort(addresses)
	return slices.Compact(addresses), nil
}

// Resolve 从当前数据源中随机选择一个地址，不添加 URL scheme 或解析 DNS。
func Resolve(ctx context.Context, serviceName string) (string, error) {
	addresses, err := Discover(ctx, serviceName)
	if err != nil {
		return "", err
	}
	return addresses[rand.IntN(len(addresses))], nil
}

func validateServiceName(name string) error {
	if name == "" || strings.ContainsAny(name, "/?# \t\r\n") {
		return errors.New("service name must not be empty or contain URL separators or spaces")
	}
	return nil
}

func validateRegistrationHost(host string) error {
	if host == "" || strings.ContainsAny(host, "/?# \t\r\n") || (strings.Contains(host, ":") && net.ParseIP(host) == nil) {
		return errors.New("expected a host name or IP without scheme or port")
	}
	return nil
}
