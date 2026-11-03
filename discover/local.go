package discover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/miebyte/goutils/logging"
)

// Local 使用不可变的本地服务目录快照；零值表示空目录，不回退到 Consul。
type Local struct {
	services map[string][]string
}

// NewLocal 读取并校验 JSON 服务目录，之后的查询不再访问文件。
func NewLocal(path string) (*Local, error) {
	services, err := loadDirectory(path)
	if err != nil {
		return nil, err
	}
	return &Local{services: services}, nil
}

// Discover 只查询本地快照，返回独立的地址切片，不检查目标连通性。
func (l *Local) Discover(ctx context.Context, serviceName string) ([]string, error) {
	serviceName = strings.TrimSpace(serviceName)
	if err := validateServiceName(serviceName); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	addresses := l.services[serviceName]
	if len(addresses) == 0 {
		return nil, fmt.Errorf("%w: service=%q (local directory)", ErrNoAvailableService, serviceName)
	}
	return slices.Clone(addresses), nil
}

func loadDirectory(path string) (map[string][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read service directory %q: %w", path, err)
	}
	var services map[string][]string
	if err := json.Unmarshal(data, &services); err != nil {
		return nil, fmt.Errorf("parse service directory %q: %w", path, err)
	}
	if services == nil {
		return nil, fmt.Errorf("service directory %q must be a JSON object", path)
	}
	for name, addresses := range services {
		if err := validateServiceName(name); err != nil || name != strings.TrimSpace(name) {
			return nil, fmt.Errorf("invalid service name %q in directory %q", name, path)
		}
		if addresses == nil {
			return nil, fmt.Errorf("service %q in directory %q must contain an address array, not null", name, path)
		}
		for i, address := range addresses {
			normalized, err := normalizeAddress(address)
			if err != nil {
				return nil, fmt.Errorf("invalid address %q for service %q in directory %q: %w", address, name, path, err)
			}
			addresses[i] = normalized
		}
		slices.Sort(addresses)
		services[name] = slices.Compact(addresses)
	}
	return services, nil
}

func normalizeAddress(address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}
	if host == "" || strings.ContainsAny(host, "/?#@\\[] \t\r\n") || ((strings.HasPrefix(address, "[") || strings.Contains(host, ":")) && net.ParseIP(host) == nil) {
		return "", errors.New("expected host:port without URL scheme, path or spaces")
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		return "", errors.New("port must be a number between 1 and 65535")
	}
	return net.JoinHostPort(host, strconv.FormatUint(portNumber, 10)), nil
}

func UseLocalSource() {
	resolver, err := NewLocal("./services.json")
	logging.PanicError(err)

	SetDefaultResolver(resolver)
}
