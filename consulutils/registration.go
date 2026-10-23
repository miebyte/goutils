package consulutils

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
	"github.com/miebyte/goutils/internal/innerlog"
)

// RegistrationConfig 配置服务入口及当前实例的健康检查。
// Client 必须连接固定的 Consul Agent，不能在多个 Agent 之间随机分发请求。
type RegistrationConfig struct {
	ServiceAddress string // 显式覆盖注册地址，只填写主机名或 IP。
	K8sServiceName string
	ClusterDomain  string
	CheckHost      string // 显式覆盖实例健康检查的主机名或 IP。
	Tags           []string
	Meta           map[string]string
	Client         *api.Client

	RequestTimeout                 time.Duration
	RetryInterval                  time.Duration
	CheckInterval                  time.Duration
	CheckTimeout                   time.Duration
	DeregisterCriticalServiceAfter time.Duration
}

// Registration 管理当前进程的注册，不拥有共享 Consul 客户端的生命周期。
type Registration struct {
	client         *api.Client
	service        *api.AgentServiceRegistration
	requestTimeout time.Duration
	retryInterval  time.Duration
	attempted      atomic.Bool
}

// NewRegistration 根据部署身份生成稳定的实例 ID；本地进程使用独立 UUID。
// serviceName 和 servicePort 是服务身份，通常由启动器从服务名和已绑定的业务 listener 获取。
// managementAddress 必须是已绑定的内部 HTTP listener 地址，健康路径固定为 /ready。
func NewRegistration(serviceName string, servicePort int, config RegistrationConfig, managementAddress string) (*Registration, error) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" || strings.ContainsAny(serviceName, "/?# \t\r\n") {
		return nil, errors.New("consul service name must not be empty or contain URL separators or spaces")
	}
	if servicePort < 1 || servicePort > 65535 {
		return nil, errors.New("consul service port must be between 1 and 65535")
	}
	_, managementPort, err := net.SplitHostPort(managementAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid management listener address: %w", err)
	}
	port, err := strconv.Atoi(managementPort)
	if err != nil || port < 1 || port > 65535 {
		return nil, errors.New("management listener must have a bound TCP port")
	}

	for _, option := range []struct {
		name         string
		value        *time.Duration
		defaultValue time.Duration
	}{
		{"request timeout", &config.RequestTimeout, 3 * time.Second},
		{"retry interval", &config.RetryInterval, 5 * time.Second},
		{"check interval", &config.CheckInterval, 10 * time.Second},
		{"check timeout", &config.CheckTimeout, 2 * time.Second},
		{"critical deregistration delay", &config.DeregisterCriticalServiceAfter, 10 * time.Minute},
	} {
		if *option.value < 0 {
			return nil, fmt.Errorf("consul %s must be positive", option.name)
		}
		if *option.value == 0 {
			*option.value = option.defaultValue
		}
	}

	namespace := strings.TrimSpace(os.Getenv("POD_NAMESPACE"))
	podUID := strings.TrimSpace(os.Getenv("POD_UID"))
	podIP := strings.TrimSpace(os.Getenv("POD_IP"))
	meta := maps.Clone(config.Meta)
	address, checkHost := "127.0.0.1", "127.0.0.1"
	instanceID := uuid.NewString()
	if namespace != "" || podUID != "" || podIP != "" {
		if namespace == "" || podUID == "" || podIP == "" {
			return nil, errors.New("POD_NAMESPACE, POD_UID and POD_IP must all be set in Kubernetes")
		}
		if net.ParseIP(podIP) == nil {
			return nil, errors.New("POD_IP must be a valid IP address")
		}
		k8sServiceName := config.K8sServiceName
		if k8sServiceName == "" {
			k8sServiceName = serviceName
		}
		if !validDNSLabel(namespace) || !validDNSLabel(k8sServiceName) {
			return nil, errors.New("Kubernetes namespace and service name must be valid DNS labels")
		}
		if strings.ContainsAny(podUID, "/?# \t\r\n") {
			return nil, errors.New("POD_UID must not contain URL separators or spaces")
		}
		domain := strings.TrimSuffix(config.ClusterDomain, ".")
		if domain == "" {
			domain = "cluster.local"
		}
		address = k8sServiceName + "." + namespace + ".svc." + domain
		checkHost = podIP
		instanceID = namespace + "-" + podUID
		if meta == nil {
			meta = make(map[string]string)
		}
		meta["namespace"] = namespace
		meta["pod_uid"] = podUID
	}
	if config.ServiceAddress != "" {
		address = config.ServiceAddress
	}
	if config.CheckHost != "" {
		checkHost = config.CheckHost
	}
	if err := validateRegistrationHost(address); err != nil {
		return nil, fmt.Errorf("invalid consul service address: %w", err)
	}
	if err := validateRegistrationHost(checkHost); err != nil {
		return nil, fmt.Errorf("invalid consul check host: %w", err)
	}
	if config.Client == nil {
		config.Client, err = DefaultClient()
		if err != nil {
			return nil, fmt.Errorf("create consul client: %w", err)
		}
	}

	id := fmt.Sprintf("%s-%d-%s", serviceName, servicePort, instanceID)
	return &Registration{
		client:         config.Client,
		requestTimeout: config.RequestTimeout,
		retryInterval:  config.RetryInterval,
		service: &api.AgentServiceRegistration{
			ID:      id,
			Name:    serviceName,
			Address: address,
			Port:    servicePort,
			Tags:    slices.Clone(config.Tags),
			Meta:    meta,
			Check: &api.AgentServiceCheck{
				CheckID:                        "service:" + id,
				HTTP:                           "http://" + net.JoinHostPort(checkHost, managementPort) + "/ready",
				Interval:                       config.CheckInterval.String(),
				Timeout:                        config.CheckTimeout.String(),
				Status:                         api.HealthCritical,
				DeregisterCriticalServiceAfter: config.DeregisterCriticalServiceAfter.String(),
			},
		},
	}, nil
}

// Run 首次注册后持续核实；仅在记录丢失时补注册，避免重置已有检查的 critical 计时。
// 网络失败采用退避重试，不终止业务服务；ctx 取消时停止请求和等待。
func (r *Registration) Run(ctx context.Context) {
	delay := r.retryInterval
	maxDelay := max(r.retryInterval, time.Minute)
	confirmed := false
	for ctx.Err() == nil {
		err := r.ensureRegistered(ctx)
		if ctx.Err() != nil {
			return
		}
		wait := r.retryInterval
		if err != nil {
			confirmed = false
			innerlog.Logger.Warnc(ctx, "Consul registration %s failed: %v", r.service.ID, err)
			wait = delay/2 + time.Duration(rand.Int64N(int64(delay-delay/2)+1))
			if delay < maxDelay/2 {
				delay *= 2
			} else {
				delay = maxDelay
			}
		} else {
			if !confirmed {
				innerlog.Logger.Infoc(ctx, "Consul registration %s confirmed at %s", r.service.ID, net.JoinHostPort(r.service.Address, strconv.Itoa(r.service.Port)))
				confirmed = true
			}
			delay = r.retryInterval
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (r *Registration) ensureRegistered(ctx context.Context) error {
	if !r.attempted.Load() {
		return r.register(ctx)
	}
	queryCtx, cancel := context.WithTimeout(ctx, r.requestTimeout)
	service, _, err := r.client.Agent().Service(r.service.ID, (&api.QueryOptions{}).WithContext(queryCtx))
	cancel()
	if err == nil && service != nil {
		r.attempted.Store(true)
		return nil
	}
	var statusError api.StatusError
	if err != nil && (!errors.As(err, &statusError) || statusError.Code != http.StatusNotFound) {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return r.register(ctx)
}

func (r *Registration) register(ctx context.Context) error {
	registerCtx, cancel := context.WithTimeout(ctx, r.requestTimeout)
	defer cancel()
	// 请求可能已被 Agent 执行，即使响应失败，退出时也必须尝试清理。
	r.attempted.Store(true)
	return r.client.Agent().ServiceRegisterOpts(r.service, (api.ServiceRegisterOpts{}).WithContext(registerCtx))
}

// Deregister 仅注销本实例。调用方应先取消并等待 Run，再传入独立的退出超时 context。
func (r *Registration) Deregister(ctx context.Context) error {
	if !r.attempted.Load() {
		return nil
	}
	requestCtx, cancel := context.WithTimeout(ctx, r.requestTimeout)
	defer cancel()
	return r.client.Agent().ServiceDeregisterOpts(r.service.ID, (&api.QueryOptions{}).WithContext(requestCtx))
}

func validateRegistrationHost(host string) error {
	if host == "" || strings.ContainsAny(host, "/?# \t\r\n") || (strings.Contains(host, ":") && net.ParseIP(host) == nil) {
		return errors.New("expected a host name or IP without scheme or port")
	}
	return nil
}

func validDNSLabel(value string) bool {
	if len(value) == 0 || len(value) > 63 || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, ch := range value {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
			return false
		}
	}
	return true
}
