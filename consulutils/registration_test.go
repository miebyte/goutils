package consulutils

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/logging"
	"github.com/stretchr/testify/require"
)

type registrationAgent struct {
	mu               sync.Mutex
	service          *api.AgentServiceRegistration
	registerFailures int
	queryFailures    int
	registerCalls    int
	queryCalls       int
	deregistered     []string
}

func (a *registrationAgent) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case req.Method == http.MethodPut && req.URL.Path == "/v1/agent/service/register":
		a.registerCalls++
		if a.registerFailures > 0 {
			a.registerFailures--
			http.Error(w, "agent unavailable", http.StatusServiceUnavailable)
			return
		}
		var service api.AgentServiceRegistration
		if err := json.NewDecoder(req.Body).Decode(&service); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.service = &service
	case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/agent/service/"):
		a.queryCalls++
		if a.queryFailures > 0 {
			a.queryFailures--
			http.Error(w, "agent unavailable", http.StatusServiceUnavailable)
			return
		}
		if a.service == nil {
			http.NotFound(w, req)
			return
		}
		_ = json.NewEncoder(w).Encode(&api.AgentService{
			ID: a.service.ID, Service: a.service.Name,
			Address: a.service.Address, Port: a.service.Port,
		})
	case req.Method == http.MethodPut && strings.HasPrefix(req.URL.Path, "/v1/agent/service/deregister/"):
		a.deregistered = append(a.deregistered, strings.TrimPrefix(req.URL.Path, "/v1/agent/service/deregister/"))
		a.service = nil
	default:
		http.NotFound(w, req)
	}
}

type registrationLogBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *registrationLogBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(value)
}

func (b *registrationLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func registrationClient(t *testing.T, handler http.Handler) *api.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	config := api.DefaultConfig()
	config.Address = server.URL
	config.HttpClient = server.Client()
	client, err := api.NewClient(config)
	require.NoError(t, err)
	return client
}

func localRegistrationConfig(t *testing.T, client *api.Client) RegistrationConfig {
	t.Helper()
	t.Setenv("POD_NAMESPACE", "")
	t.Setenv("POD_UID", "")
	t.Setenv("POD_IP", "")
	return RegistrationConfig{
		Client:        client,
		RetryInterval: 5 * time.Millisecond, RequestTimeout: 100 * time.Millisecond,
	}
}

func runRegistration(t *testing.T, registration *Registration) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		registration.Run(ctx)
		close(done)
	}()
	stop := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("registration did not stop after cancellation")
		}
	}
	t.Cleanup(stop)
	return stop
}

func TestRegistrationRetriesAndRestoresMissingService(t *testing.T) {
	agent := &registrationAgent{registerFailures: 1}
	config := localRegistrationConfig(t, registrationClient(t, agent))
	registration, err := NewRegistration("orders", 8080, config, "[::]:6060")
	require.NoError(t, err)
	stop := runRegistration(t, registration)
	require.Eventually(t, func() bool {
		agent.mu.Lock()
		defer agent.mu.Unlock()
		return agent.service != nil && agent.queryCalls >= 5
	}, time.Second, time.Millisecond)
	agent.mu.Lock()
	registerCalls, service := agent.registerCalls, agent.service
	agent.service = nil // 模拟 Agent 重启或 critical 超时自动注销。
	agent.mu.Unlock()
	require.Equal(t, 2, registerCalls, "已有记录时不能反复注册并重置检查状态")
	require.Equal(t, registration.service.ID, service.ID)
	require.Equal(t, api.HealthCritical, service.Check.Status)
	require.Eventually(t, func() bool {
		agent.mu.Lock()
		defer agent.mu.Unlock()
		return agent.service != nil && agent.registerCalls == 3
	}, time.Second, time.Millisecond)
	stop()
	require.NoError(t, registration.Deregister(context.Background()))
	agent.mu.Lock()
	defer agent.mu.Unlock()
	require.Equal(t, []string{registration.service.ID}, agent.deregistered)
}

func TestRegistrationLogsInitialConfirmationAndRecovery(t *testing.T) {
	var output registrationLogBuffer
	originalLogger := innerlog.Logger
	innerlog.Logger = logging.NewPrettyLogger(&output)
	t.Cleanup(func() { innerlog.Logger = originalLogger })
	agent := &registrationAgent{}
	config := localRegistrationConfig(t, registrationClient(t, agent))
	registration, err := NewRegistration("orders", 8080, config, ":6060")
	require.NoError(t, err)
	stop := runRegistration(t, registration)
	require.Eventually(t, func() bool {
		agent.mu.Lock()
		defer agent.mu.Unlock()
		return agent.queryCalls >= 3
	}, time.Second, time.Millisecond)
	require.Equal(t, 1, strings.Count(output.String(), "confirmed at"))
	agent.mu.Lock()
	agent.queryFailures = 1
	queryCalls := agent.queryCalls
	agent.mu.Unlock()
	require.Eventually(t, func() bool {
		agent.mu.Lock()
		defer agent.mu.Unlock()
		return agent.queryCalls >= queryCalls+4
	}, time.Second, time.Millisecond)
	stop()
	require.Contains(t, output.String(), "failed:")
	require.Equal(t, 2, strings.Count(output.String(), "confirmed at"), "持续核实成功时不应重复输出日志")
}

func TestRegistrationUpdatesExistingRecordOnlyAtStartup(t *testing.T) {
	agent := &registrationAgent{}
	config := localRegistrationConfig(t, registrationClient(t, agent))
	config.ServiceAddress = "new.example"
	registration, err := NewRegistration("orders", 8080, config, ":6060")
	require.NoError(t, err)
	agent.service = &api.AgentServiceRegistration{ID: registration.service.ID, Address: "old.example"}
	require.NoError(t, registration.ensureRegistered(context.Background()))
	require.NoError(t, registration.ensureRegistered(context.Background()))
	agent.mu.Lock()
	defer agent.mu.Unlock()
	require.Equal(t, "new.example", agent.service.Address)
	require.Equal(t, 1, agent.registerCalls)
	require.Equal(t, 1, agent.queryCalls)
}

func TestRegistrationConfirmsUnknownRegisterResultWithoutRepeatingPut(t *testing.T) {
	agent := &registrationAgent{}
	client := registrationClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		agent.ServeHTTP(w, req)
		if req.URL.Path == "/v1/agent/service/register" {
			// 模拟 Agent 已保存记录，但客户端只收到错误响应。
			http.Error(w, "response unavailable", http.StatusServiceUnavailable)
		}
	}))
	config := localRegistrationConfig(t, client)
	registration, err := NewRegistration("orders", 8080, config, ":6060")
	require.NoError(t, err)
	stop := runRegistration(t, registration)
	require.Eventually(t, func() bool {
		agent.mu.Lock()
		defer agent.mu.Unlock()
		return agent.service != nil && agent.queryCalls >= 3
	}, time.Second, time.Millisecond)
	stop()
	agent.mu.Lock()
	defer agent.mu.Unlock()
	require.Equal(t, 1, agent.registerCalls)
}

func TestRegistrationAddressesAndInstanceIDs(t *testing.T) {
	config := localRegistrationConfig(t, registrationClient(t, &registrationAgent{}))
	first, err := NewRegistration("orders", 8080, config, "127.0.0.1:6060")
	require.NoError(t, err)
	second, err := NewRegistration("orders", 8080, config, "127.0.0.1:6060")
	require.NoError(t, err)
	require.NotEqual(t, first.service.ID, second.service.ID)
	require.Equal(t, "127.0.0.1", first.service.Address)
	require.Equal(t, "http://127.0.0.1:6060/ready", first.service.Check.HTTP)
	require.Equal(t, "10s", first.service.Check.Interval)
	require.Equal(t, "2s", first.service.Check.Timeout)
	require.Equal(t, "10m0s", first.service.Check.DeregisterCriticalServiceAfter)
	require.Empty(t, first.service.Meta)

	t.Setenv("POD_NAMESPACE", "shop")
	t.Setenv("POD_UID", "pod-one")
	t.Setenv("POD_IP", "10.0.0.8")
	first, err = NewRegistration("orders", 8080, config, "[::]:6060")
	require.NoError(t, err)
	require.Equal(t, "orders", first.service.Name)
	require.Equal(t, "orders.shop.svc.cluster.local", first.service.Address)
	require.Equal(t, 8080, first.service.Port)
	config.K8sServiceName = "orders-http"
	config.ClusterDomain = "internal.example"
	config.Meta = map[string]string{"namespace": "wrong", "pod_uid": "wrong", "team": "shop"}
	first, err = NewRegistration("orders", 80, config, "[::]:16060")
	require.NoError(t, err)
	require.Equal(t, "orders-http.shop.svc.internal.example", first.service.Address)
	require.Equal(t, 80, first.service.Port)
	require.Equal(t, "http://10.0.0.8:16060/ready", first.service.Check.HTTP)
	require.Equal(t, map[string]string{"namespace": "shop", "pod_uid": "pod-one", "team": "shop"}, first.service.Meta)
	require.Equal(t, "wrong", config.Meta["namespace"], "不能修改调用方的配置映射")
	second, err = NewRegistration("orders", 80, config, "[::]:16060")
	require.NoError(t, err)
	require.Equal(t, first.service.ID, second.service.ID)
	t.Setenv("POD_UID", "pod-two")
	second, err = NewRegistration("orders", 80, config, "[::]:16060")
	require.NoError(t, err)
	require.NotEqual(t, first.service.ID, second.service.ID)
	require.Equal(t, first.service.Address, second.service.Address)

	config.ServiceAddress = "entry.example"
	config.CheckHost = "2001:db8::1"
	first, err = NewRegistration("orders", 80, config, ":16060")
	require.NoError(t, err)
	require.Equal(t, "entry.example", first.service.Address)
	require.Equal(t, "http://[2001:db8::1]:16060/ready", first.service.Check.HTTP)
}

func TestRegistrationRequiresServiceIdentity(t *testing.T) {
	config := localRegistrationConfig(t, registrationClient(t, &registrationAgent{}))
	for _, test := range []struct {
		name        string
		serviceName string
		servicePort int
	}{
		{name: "missing name", servicePort: 8080},
		{name: "invalid name", serviceName: "orders/api", servicePort: 8080},
		{name: "missing port", serviceName: "orders"},
		{name: "invalid port", serviceName: "orders", servicePort: 65536},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRegistration(test.serviceName, test.servicePort, config, ":6060")
			require.Error(t, err)
		})
	}
}

func TestRegistrationRejectsIncompleteKubernetesIdentity(t *testing.T) {
	config := localRegistrationConfig(t, registrationClient(t, &registrationAgent{}))
	for _, missing := range []string{"POD_NAMESPACE", "POD_UID", "POD_IP"} {
		t.Run(missing, func(t *testing.T) {
			t.Setenv("POD_NAMESPACE", "shop")
			t.Setenv("POD_UID", "pod-one")
			t.Setenv("POD_IP", "10.0.0.8")
			t.Setenv(missing, "")
			_, err := NewRegistration("orders", 8080, config, ":6060")
			require.ErrorContains(t, err, "must all be set")
		})
	}
}

func TestRegistrationCancelsInflightRequest(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	client := registrationClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = io.Copy(io.Discard, req.Body)
		close(started)
		<-req.Context().Done()
		close(canceled)
	}))
	config := localRegistrationConfig(t, client)
	config.RequestTimeout = time.Minute
	registration, err := NewRegistration("orders", 8080, config, ":6060")
	require.NoError(t, err)
	stop := runRegistration(t, registration)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("registration did not send a request")
	}
	stop()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("request context was not canceled")
	}
}

func TestRegistrationRequestTimeoutRetries(t *testing.T) {
	agent := &registrationAgent{}
	var requests atomic.Int32
	client := registrationClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if requests.Add(1) == 1 {
			_, _ = io.Copy(io.Discard, req.Body)
			<-req.Context().Done()
			return
		}
		agent.ServeHTTP(w, req)
	}))
	config := localRegistrationConfig(t, client)
	config.RequestTimeout = 10 * time.Millisecond
	registration, err := NewRegistration("orders", 8080, config, ":6060")
	require.NoError(t, err)
	runRegistration(t, registration)
	require.Eventually(t, func() bool {
		agent.mu.Lock()
		defer agent.mu.Unlock()
		return agent.service != nil
	}, time.Second, time.Millisecond)
}

func TestRegistrationCleansUpUnknownRegisterResult(t *testing.T) {
	started := make(chan struct{})
	deregistered := make(chan string, 1)
	client := registrationClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet:
			http.NotFound(w, req)
		case req.URL.Path == "/v1/agent/service/register":
			// Agent 可能已完成注册，但响应在客户端取消前未返回。
			var service api.AgentServiceRegistration
			_ = json.NewDecoder(req.Body).Decode(&service)
			close(started)
			<-req.Context().Done()
		case strings.HasPrefix(req.URL.Path, "/v1/agent/service/deregister/"):
			deregistered <- strings.TrimPrefix(req.URL.Path, "/v1/agent/service/deregister/")
		}
	}))
	config := localRegistrationConfig(t, client)
	config.RequestTimeout = time.Minute
	registration, err := NewRegistration("orders", 8080, config, ":6060")
	require.NoError(t, err)
	stop := runRegistration(t, registration)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("registration did not start")
	}
	stop()
	require.NoError(t, registration.Deregister(context.Background()))
	select {
	case id := <-deregistered:
		require.Equal(t, registration.service.ID, id)
	default:
		t.Fatal("uncertain registration was not cleaned up")
	}
}

func TestRegistrationDeregisterHonorsCancellation(t *testing.T) {
	agent := &registrationAgent{}
	config := localRegistrationConfig(t, registrationClient(t, agent))
	registration, err := NewRegistration("orders", 8080, config, ":6060")
	require.NoError(t, err)
	require.NoError(t, registration.ensureRegistered(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, registration.Deregister(ctx), context.Canceled)
	agent.mu.Lock()
	defer agent.mu.Unlock()
	require.Empty(t, agent.deregistered)
}
