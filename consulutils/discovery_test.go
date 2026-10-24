package consulutils

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestDiscoverUsesHealthyServiceAddresses(t *testing.T) {
	t.Setenv("CONSUL_NAMESPACE", "")
	client := registrationClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/health/service/orders" || r.URL.Query().Get("passing") != "1" {
			t.Errorf("health query = %s %s", r.Method, r.URL)
		}
		if r.URL.Query().Get("ns") != "" || r.URL.Query().Get("tag") != "" {
			t.Errorf("unexpected enterprise namespace or tag in health query: %s", r.URL)
		}
		_ = json.NewEncoder(w).Encode([]*api.ServiceEntry{{
			Node:    &api.Node{Address: "wrong-node.example"},
			Service: &api.AgentService{Address: "orders.prod.svc", Port: 8080},
		}})
	}))
	addresses, err := Discover(context.Background(), "orders", DiscoveryConfig{Client: client})
	require.NoError(t, err)
	require.Equal(t, []string{"orders.prod.svc:8080"}, addresses)
}

func TestDiscoverNamespaceSelection(t *testing.T) {
	// 默认发现不读取调用方 Pod 的 namespace，允许同一 Consul 返回不同环境的记录。
	t.Setenv("POD_NAMESPACE", "caller")
	t.Setenv("CONSUL_NAMESPACE", "")
	entries := []*api.ServiceEntry{
		{Service: &api.AgentService{Address: "orders.prod.svc", Port: 8080, Meta: map[string]string{"namespace": "prod"}}},
		{Service: &api.AgentService{Address: "orders.test.svc", Port: 8080, Meta: map[string]string{"namespace": "test"}}},
		{Service: &api.AgentService{Address: "127.0.0.1", Port: 8080}},
		{Service: &api.AgentService{Address: "127.0.0.1", Port: 8081, Meta: map[string]string{"team": "shop"}}},
		{Service: &api.AgentService{Address: "127.0.0.1", Port: 8082, Meta: map[string]string{"namespace": ""}}},
	}
	client := registrationClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ns") != "" {
			t.Errorf("Kubernetes namespace was sent as Consul Enterprise namespace: %s", r.URL)
		}
		_ = json.NewEncoder(w).Encode(entries)
	}))
	all := []string{"127.0.0.1:8080", "127.0.0.1:8081", "127.0.0.1:8082", "orders.prod.svc:8080", "orders.test.svc:8080"}
	for _, test := range []struct {
		name      string
		namespace string
		want      []string
	}{
		{name: "default includes all namespaces", want: all},
		{name: "whitespace means no filter", namespace: " \t ", want: all},
		{name: "explicit namespace is trimmed", namespace: " prod ", want: []string{"orders.prod.svc:8080"}},
		{name: "exact namespace match", namespace: "PROD"},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := DiscoveryConfig{Namespace: test.namespace, Client: client}
			addresses, err := Discover(context.Background(), "orders", config)
			if len(test.want) == 0 {
				require.ErrorIs(t, err, ErrNoAvailableService)
				require.Empty(t, addresses)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.want, addresses)
			}
			require.Equal(t, test.namespace, config.Namespace, "不能原地修改调用方的配置")
		})
	}
}

func TestDiscoverValidatesAndDeduplicatesAddresses(t *testing.T) {
	entries := []*api.ServiceEntry{
		nil, {},
		{Node: &api.Node{Address: "must-not-fallback.example"}, Service: &api.AgentService{Port: 8080}},
		{Service: &api.AgentService{Address: "orders.prod.svc", Port: 8080}},
		{Service: &api.AgentService{Address: "orders.prod.svc", Port: 8080}},
		{Service: &api.AgentService{Address: "orders.prod.svc", Port: 9090}},
		{Service: &api.AgentService{Address: "2001:db8::1", Port: 8080}},
		{Service: &api.AgentService{Address: "127.0.0.1", Port: 65535}},
	}
	for _, address := range []string{"", "http://orders.prod.svc", "orders.prod.svc/path", "orders?bad", "orders#bad", "orders prod", "orders:8080", "[2001:db8::1]"} {
		entries = append(entries, &api.ServiceEntry{Service: &api.AgentService{Address: address, Port: 8080}})
	}
	for _, port := range []int{-1, 0, 65536} {
		entries = append(entries, &api.ServiceEntry{Service: &api.AgentService{Address: "invalid-port.example", Port: port}})
	}
	client := registrationClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(entries)
	}))
	client.AddHeader("X-Caller-Header", "preserve")
	headers := client.Headers()
	addresses, err := Discover(context.Background(), "orders", DiscoveryConfig{Client: client})
	require.NoError(t, err)
	require.Equal(t, []string{"127.0.0.1:65535", "[2001:db8::1]:8080", "orders.prod.svc:8080", "orders.prod.svc:9090"}, addresses)
	require.Equal(t, headers, client.Headers(), "发现不能改写共用 client 的请求头")
}

func TestDiscoverReturnsNoAvailableService(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		acl  bool
	}{
		{name: "empty result", body: `[]`},
		{name: "null result", body: `null`},
		{name: "invalid entries only", body: `[{"Service":{"Address":"","Port":8080}}]`},
		{name: "ACL filtered empty result", body: `[]`, acl: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var requests atomic.Int32
			client := registrationClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				// 模拟真实查询元数据，SDK 读到 LastContact 后才会继续解析 ACL 标记。
				w.Header().Set("X-Consul-LastContact", "0")
				if test.acl {
					w.Header().Set("X-Consul-Results-Filtered-By-ACLs", "true")
				}
				_, _ = w.Write([]byte(test.body))
			}))
			addresses, err := Discover(context.Background(), "orders", DiscoveryConfig{Client: client})
			require.ErrorIs(t, err, ErrNoAvailableService)
			if test.acl {
				require.ErrorContains(t, err, "results filtered by ACLs")
			}
			require.Empty(t, addresses)
			require.EqualValues(t, 1, requests.Load(), "没有可用实例时不能在库内重试")
		})
	}
}

func TestDiscoverRejectsInvalidArguments(t *testing.T) {
	var requests atomic.Int32
	client := registrationClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`[]`))
	}))
	config := DiscoveryConfig{Client: client}
	for _, name := range []string{"", " \t ", "orders/path", "orders?bad", "orders#bad", "orders name", "orders\tname"} {
		t.Run(name, func(t *testing.T) {
			_, err := Discover(context.Background(), name, config)
			require.Error(t, err)
			require.NotErrorIs(t, err, ErrNoAvailableService)
		})
	}
	_, err := Discover(context.Background(), "orders", config, config)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrNoAvailableService)
	require.Zero(t, requests.Load(), "参数错误不能访问 Consul")
}

func TestDiscoverPropagatesHTTPFailuresWithoutRetry(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "permission denied", status: http.StatusForbidden, body: "permission denied"},
		{name: "unavailable", status: http.StatusServiceUnavailable, body: "Consul unavailable"},
		{name: "invalid JSON", status: http.StatusOK, body: "not JSON"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var requests atomic.Int32
			client := registrationClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			_, err := Discover(context.Background(), "orders", DiscoveryConfig{Client: client})
			require.Error(t, err)
			require.NotErrorIs(t, err, ErrNoAvailableService)
			if test.status != http.StatusOK {
				var statusError api.StatusError
				require.ErrorAs(t, err, &statusError)
				require.Equal(t, test.status, statusError.Code)
			}
			require.EqualValues(t, 1, requests.Load())
		})
	}
}

type discoveryRoundTripper func(*http.Request) (*http.Response, error)

func (fn discoveryRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestDiscoverBoundsAndPreservesRequestContext(t *testing.T) {
	for _, test := range []struct {
		name    string
		timeout time.Duration
	}{
		{name: "default three second maximum"},
		{name: "shorter caller deadline", timeout: 100 * time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			if test.timeout != 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, test.timeout)
				defer cancel()
			}
			var observed context.Context
			var requests int
			wanted := errors.New("transport unavailable")
			config := api.DefaultConfig()
			config.HttpClient = &http.Client{Transport: discoveryRoundTripper(func(request *http.Request) (*http.Response, error) {
				requests++
				observed = request.Context()
				return nil, wanted
			})}
			client, err := api.NewClient(config)
			require.NoError(t, err)
			started := time.Now()
			_, err = Discover(ctx, "orders", DiscoveryConfig{Client: client})
			require.ErrorIs(t, err, wanted, "网络错误必须可透过包装识别")
			require.Equal(t, 1, requests, "网络错误不能在库内重试")
			require.NotNil(t, observed)
			deadline, ok := observed.Deadline()
			require.True(t, ok, "没有调用方 deadline 时也必须限制单次发现请求")
			if test.timeout == 0 {
				require.WithinDuration(t, started.Add(3*time.Second), deadline, 100*time.Millisecond)
			} else {
				callerDeadline, _ := ctx.Deadline()
				require.Equal(t, callerDeadline, deadline)
			}
			select {
			case <-observed.Done():
			default:
				t.Fatal("request context was not released after discovery")
			}
		})
	}
}

func TestDiscoverRespectsCancellation(t *testing.T) {
	for _, mode := range []string{"already canceled", "cancel in flight", "caller deadline"} {
		t.Run(mode, func(t *testing.T) {
			started := make(chan struct{}, 1)
			var requests atomic.Int32
			client := registrationClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				started <- struct{}{}
				<-r.Context().Done()
			}))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "already canceled" {
				cancel()
			}
			if mode == "caller deadline" {
				var stop context.CancelFunc
				ctx, stop = context.WithTimeout(ctx, 100*time.Millisecond)
				defer stop()
			}
			result := make(chan error, 1)
			go func() {
				_, err := Discover(ctx, "orders", DiscoveryConfig{Client: client})
				result <- err
			}()
			if mode == "cancel in flight" {
				select {
				case <-started:
					cancel()
				case <-time.After(time.Second):
					t.Fatal("discovery did not start the HTTP request")
				}
			}
			select {
			case err := <-result:
				if mode == "caller deadline" {
					require.ErrorIs(t, err, context.DeadlineExceeded)
				} else {
					require.ErrorIs(t, err, context.Canceled)
				}
				require.NotErrorIs(t, err, ErrNoAvailableService)
			case <-time.After(time.Second):
				t.Fatal("discovery ignored cancellation or caller deadline")
			}
			if mode == "already canceled" {
				require.Zero(t, requests.Load())
			} else {
				require.EqualValues(t, 1, requests.Load())
			}
		})
	}
}

func TestResolveReturnsDiscoveredMember(t *testing.T) {
	var requests atomic.Int32
	client := registrationClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.WriteString(w, `[{"Service":{"Address":"127.0.0.1","Port":8080}},{"Service":{"Address":"127.0.0.1","Port":8080}},{"Service":{"Address":"orders.prod.svc","Port":8081}}]`)
	}))
	for range 12 {
		address, err := Resolve(context.Background(), "orders", DiscoveryConfig{Client: client})
		require.NoError(t, err)
		require.Contains(t, []string{"127.0.0.1:8080", "orders.prod.svc:8081"}, address)
	}
	// 只校验成员合法，不要求随机结果达到某种频次；每次调用按需查一次，没有隐式缓存。
	require.EqualValues(t, 12, requests.Load())
}

func TestResolvePreservesDiscoveryErrors(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := registrationClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `[]`)
			}))
			address, err := Resolve(context.Background(), "orders", DiscoveryConfig{Client: client})
			require.Empty(t, address)
			if status == http.StatusOK {
				require.ErrorIs(t, err, ErrNoAvailableService)
			} else {
				var statusError api.StatusError
				require.ErrorAs(t, err, &statusError)
				require.Equal(t, status, statusError.Code)
			}
		})
	}
}
