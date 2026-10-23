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
	Tags           []string
	Meta           map[string]string
	Client         *api.Client

	RequestTimeout                 time.Duration
	RetryInterval                  time.Duration
	CheckInterval                  time.Duration
	CheckTimeout                   time.Duration
	DeregisterCriticalServiceAfter time.Duration
}

func getServicePort(address string) (int, error) {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return 0, fmt.Errorf("invalid address: %w", err)
	}

	portInt, err := strconv.Atoi(port)
	if err != nil {
		return 0, fmt.Errorf("invalid port: %w", err)
	}

	if portInt < 1 || portInt > 65535 {
		return 0, errors.New("port must be between 1 and 65535")
	}

	return portInt, nil
}

func supplyDefaultConfig(config *RegistrationConfig) error {
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
			return fmt.Errorf("consul %s must be positive", option.name)
		}
		if *option.value == 0 {
			*option.value = option.defaultValue
		}
	}

	return nil
}

// NewRegistration 在集群中用命名空间、Pod 名和业务端口生成实例 ID；本地进程使用独立 UUID。
// serviceName 来自进程服务名，businessAddress 和 managementAddress 使用已绑定的监听地址。
// 健康检查固定使用当前实例的管理端口和 /ready，不跟随注册地址覆盖。
func NewRegistration(serviceName string, businessAddress string, managementAddress string, config RegistrationConfig) (*Registration, error) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" || strings.ContainsAny(serviceName, "/?# \t\r\n") {
		return nil, errors.New("consul service name must not be empty or contain URL separators or spaces")
	}

	businessPort, err := getServicePort(businessAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid business listener address: %w", err)
	}

	managementPort, err := getServicePort(managementAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid management listener address: %w", err)
	}

	err = supplyDefaultConfig(&config)
	if err != nil {
		return nil, fmt.Errorf("supply default config: %w", err)
	}

	// 尝试获取 k8s 集群的环境变量
	namespace := strings.TrimSpace(os.Getenv("POD_NAMESPACE"))
	podName := strings.TrimSpace(os.Getenv("POD_NAME"))
	podIP := strings.TrimSpace(os.Getenv("POD_IP"))

	// 复制一份元信息
	meta := maps.Clone(config.Meta)

	// 默认使用 127.0.0.1 作为服务地址和检查主机
	address, checkHost := "127.0.0.1", "127.0.0.1"

	// 本地进程通过独立 UUID 区分，即使服务名和端口相同也不会覆盖彼此。
	id := fmt.Sprintf("%s-%d-%s", serviceName, businessPort, uuid.NewString())

	// 任何一个集群环境变量被设置，都认为在 k8s 集群中运行
	// 但是必须三个变量同时被设置才会允许继续注册
	if namespace != "" || podName != "" || podIP != "" {
		if namespace == "" || podName == "" || podIP == "" {
			return nil, errors.New("POD_NAMESPACE, POD_NAME and POD_IP must all be set in Kubernetes")
		}

		if net.ParseIP(podIP) == nil {
			return nil, errors.New("POD_IP must be a valid IP address")
		}

		if !validDNSLabel(namespace) || !validDNSLabel(serviceName) {
			return nil, errors.New("Kubernetes namespace and service name must be valid DNS labels")
		}

		if strings.ContainsAny(podName, "/?# \t\r\n") {
			return nil, errors.New("POD_NAME must not contain URL separators or spaces")
		}
		// 集群内统一按进程服务名和标准集群域生成地址，健康检查始终指向当前 Pod。
		address = serviceName + "." + namespace + ".svc.cluster.local"
		checkHost = podIP
		// 服务名仍用于服务发现；实例 ID 单独区分命名空间、Pod 和业务端口。
		id = fmt.Sprintf("%s-%s-%d", namespace, podName, businessPort)
		if meta == nil {
			meta = make(map[string]string)
		}
		meta["namespace"] = namespace
		meta["pod_name"] = podName
	}

	// 如果显式覆盖了服务地址，则使用覆盖的地址
	if config.ServiceAddress != "" {
		address = config.ServiceAddress
	}

	// 验证服务地址是否合法
	if err := validateRegistrationHost(address); err != nil {
		return nil, fmt.Errorf("invalid consul service address: %w", err)
	}

	if config.Client == nil {
		config.Client, err = DefaultClient()
		if err != nil {
			return nil, fmt.Errorf("create consul client: %w", err)
		}
	}

	return &Registration{
		client:         config.Client,
		requestTimeout: config.RequestTimeout,
		retryInterval:  config.RetryInterval,
		service: &api.AgentServiceRegistration{
			ID:      id,
			Name:    serviceName,
			Address: address,
			Port:    businessPort,
			Tags:    slices.Clone(config.Tags),
			Meta:    meta,
			Check: &api.AgentServiceCheck{
				CheckID:                        "service:" + id,
				HTTP:                           "http://" + net.JoinHostPort(checkHost, strconv.Itoa(managementPort)) + "/ready",
				Interval:                       config.CheckInterval.String(),
				Timeout:                        config.CheckTimeout.String(),
				Status:                         api.HealthCritical,
				DeregisterCriticalServiceAfter: config.DeregisterCriticalServiceAfter.String(),
			},
		},
	}, nil
}

// Registration 管理当前进程的注册，不拥有共享 Consul 客户端的生命周期。
type Registration struct {
	client         *api.Client
	service        *api.AgentServiceRegistration
	requestTimeout time.Duration
	retryInterval  time.Duration
	attempted      atomic.Bool
}

// Run 首次注册后持续核实；仅在记录丢失时补注册，避免重置已有检查的 critical 计时。
// 网络失败采用退避重试，不终止业务服务；ctx 取消时停止请求和等待。
func (r *Registration) Run(ctx context.Context) {
	// 失败时从重试间隔开始退避，单次等待最长不超过一分钟。
	delay := r.retryInterval
	maxDelay := max(r.retryInterval, time.Minute)

	// 只在状态从未确认变为已确认时打日志。
	confirmed := false
	for ctx.Err() == nil {
		// 确保当前实例已注册
		err := r.ensureRegistered(ctx)
		// 核实过程中被取消时直接退出，不再安排下一轮等待。
		if ctx.Err() != nil {
			return
		}

		wait := r.retryInterval
		if err != nil {
			confirmed = false
			innerlog.Logger.Warnc(ctx, "Consul registration %s failed: %v", r.service.ID, err)
			// 在当前窗口的一半到全长之间随机等待，错开多个实例的重试。
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
			// 本轮成功后恢复初始间隔，避免沿用失败时拉长的窗口。
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
	// 如果还没有尝试过注册，则尝试注册
	if !r.attempted.Load() {
		return r.register(ctx)
	}

	// 尝试查询当前实例是否已注册
	queryCtx, cancel := context.WithTimeout(ctx, r.requestTimeout)
	service, _, err := r.client.Agent().Service(r.service.ID, (&api.QueryOptions{}).WithContext(queryCtx))
	cancel()

	// 如果查询成功且实例存在，则认为已注册
	if err == nil && service != nil {
		r.attempted.Store(true)
		return nil
	}

	// Consul 返回的是值类型 StatusError。404 和空结果都表示记录已丢失。
	if err != nil {
		statusError, ok := errors.AsType[api.StatusError](err)
		if !ok || statusError.Code != http.StatusNotFound {
			return fmt.Errorf("consul service lookup failed: %w", err)
		}
	}

	// 上下文已取消时不再补注册。
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

	// 注册服务
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
