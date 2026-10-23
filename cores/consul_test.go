package cores

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/miebyte/goutils/consulutils"
	"github.com/miebyte/goutils/share"
)

func testConsulClient(t *testing.T, handler http.Handler) *consulapi.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	config := consulapi.DefaultConfig()
	config.Address = server.URL
	config.Token = ""
	client, err := consulapi.NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func clearPodEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"POD_NAMESPACE", "POD_NAME", "POD_IP"} {
		t.Setenv(name, "")
	}
}

func setConsulTestServiceName(t *testing.T, name string) {
	t.Helper()
	original := share.GetServiceName()
	share.SetServiceName(name)
	t.Cleanup(func() { share.SetServiceName(original) })
}

func TestConsulFailureRetriesWithoutStoppingService(t *testing.T) {
	clearPodEnvironment(t)
	setConsulTestServiceName(t, "orders")
	var mu sync.Mutex
	var entry *consulapi.AgentServiceRegistration
	var attempts int
	failed := make(chan struct{}, 1)
	registered := make(chan struct{}, 1)
	deregistered := make(chan int, 1)
	client := testConsulClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/v1/agent/service/register":
			attempts++
			if attempts == 1 {
				failed <- struct{}{}
				http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
			var registration consulapi.AgentServiceRegistration
			if err := json.NewDecoder(r.Body).Decode(&registration); err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			entry = &registration
			registered <- struct{}{}
		case strings.HasPrefix(r.URL.Path, "/v1/agent/service/deregister/"):
			if entry == nil || !strings.HasSuffix(r.URL.Path, "/"+entry.ID) {
				t.Error("deregister did not target the current instance")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			// 注销时管理端口仍在，但就绪检查已经失败；不能先关闭 HTTP 再注销。
			healthClient := &http.Client{Timeout: time.Second}
			defer healthClient.CloseIdleConnections()
			response, err := healthClient.Get(entry.Check.HTTP)
			status := 0
			if err == nil {
				status = response.StatusCode
				_ = response.Body.Close()
			} else {
				t.Errorf("management HTTP closed before deregistration: %v", err)
			}
			entry = nil
			deregistered <- status
		case strings.HasPrefix(r.URL.Path, "/v1/agent/service/"):
			if entry == nil {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(entry)
		default:
			http.NotFound(w, r)
		}
	}))
	running := startTestService(t,
		WithServiceAddress("127.0.0.1:0"),
		WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.NotFoundHandler()}),
		WithConsulRegister(consulutils.RegistrationConfig{
			Client: client, RetryInterval: 100 * time.Millisecond, RequestTimeout: time.Second,
		}),
	)
	select {
	case <-failed:
	case <-time.After(serviceTestTimeout):
		t.Fatal("registration did not reach Consul")
	}
	assertHttpResponse(t, running.httpAddr, "/ready", http.StatusOK, "ok")
	select {
	case <-registered:
	case <-running.finished:
		t.Fatalf("service stopped because Consul failed: %v", running.err)
	case <-time.After(serviceTestTimeout):
		t.Fatal("registration did not recover")
	}
	mu.Lock()
	registration := *entry
	mu.Unlock()
	assertConsulServiceIdentity(t, running, &registration, running.businessAddrs[0])
	running.service.cancel()
	if err := running.wait(t); err != nil {
		t.Fatal(err)
	}
	select {
	case status := <-deregistered:
		if status != http.StatusServiceUnavailable {
			t.Fatalf("readiness during deregistration = %d, want 503", status)
		}
	default:
		t.Fatal("service returned without deregistering")
	}
}

func TestRunSkipsManagementAndConsul(t *testing.T) {
	var requests atomic.Int32
	client := testConsulClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	service := NewCores(
		WithInternalHTTPAddress("invalid management address"),
		WithPprof(),
		WithConsulRegister(consulutils.RegistrationConfig{Client: client}),
		WithWorker(func(context.Context) error { return nil }),
	)
	if err := Run(service); err != nil {
		t.Fatal(err)
	}
	if service.internalHTTP.listener != nil || requests.Load() != 0 {
		t.Fatal("script mode started management HTTP or contacted Consul")
	}
}

func TestInvalidConsulEnvironmentDoesNotStopService(t *testing.T) {
	clearPodEnvironment(t)
	setConsulTestServiceName(t, "orders")
	t.Setenv("POD_NAMESPACE", "test")
	running := startTestService(t,
		WithServiceAddress("127.0.0.1:0"),
		WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.NotFoundHandler()}),
		WithConsulRegister(),
	)
	assertHttpResponse(t, running.httpAddr, "/ready", http.StatusOK, "ok")
	running.service.cancel()
	if err := running.wait(t); err != nil {
		t.Fatal(err)
	}
}

func assertConsulServiceIdentity(t *testing.T, running *testService, registration *consulapi.AgentServiceRegistration, businessAddress string) {
	t.Helper()
	_, portText, err := net.SplitHostPort(businessAddress)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	if registration.Name != "orders" || registration.Port != port || registration.Address != "127.0.0.1" {
		t.Fatalf("registration identity = %s:%d at %s, want orders:%d at 127.0.0.1", registration.Name, registration.Port, registration.Address, port)
	}
	if registration.Check.HTTP != "http://"+running.httpAddr+"/ready" {
		t.Fatalf("health check = %q, want management address %s", registration.Check.HTTP, running.httpAddr)
	}
	if registration.Check.HTTP == "http://"+businessAddress+"/ready" {
		t.Fatal("registration used business port for management health check")
	}
}

func TestConsulRegistersGrpcBusinessPort(t *testing.T) {
	for _, mode := range []string{"grpc", "http and grpc"} {
		t.Run(mode, func(t *testing.T) {
			clearPodEnvironment(t)
			setConsulTestServiceName(t, "orders")
			registered := make(chan *consulapi.AgentServiceRegistration, 1)
			polled := make(chan struct{}, 1)
			var registerCount, queryCount atomic.Int32
			var mu sync.Mutex
			var entry *consulapi.AgentServiceRegistration
			client := testConsulClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				switch {
				case r.URL.Path == "/v1/agent/service/register":
					var registration consulapi.AgentServiceRegistration
					if err := json.NewDecoder(r.Body).Decode(&registration); err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					entry = &registration
					registerCount.Add(1)
					select {
					case registered <- entry:
					default:
					}
				case r.Method == http.MethodGet:
					if entry == nil {
						http.NotFound(w, r)
						return
					}
					_ = json.NewEncoder(w).Encode(entry)
					if queryCount.Add(1) == 3 {
						polled <- struct{}{}
					}
				}
			}))
			opts := []ServiceOption{
				WithServiceAddress("127.0.0.1:0"),
				WithGrpcServer(newHealthServer()),
				WithConsulRegister(consulutils.RegistrationConfig{Client: client, RetryInterval: 5 * time.Millisecond}),
			}
			if mode == "http and grpc" {
				opts = append(opts, WithHttpHandler(HttpHandler{
					ApiPrefix: "/api", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						_, _ = w.Write([]byte("mixed"))
					}),
				}))
			}
			running := startTestService(t, opts...)
			select {
			case registration := <-registered:
				assertConsulServiceIdentity(t, running, registration, running.grpcAddr)
			case <-time.After(serviceTestTimeout):
				t.Fatal("gRPC service did not register")
			}
			checkHealth(t, healthClient(t, running.grpcAddr))
			if mode == "http and grpc" {
				if len(running.businessAddrs) != 1 || running.businessAddrs[0] != running.grpcAddr {
					t.Fatalf("HTTP and gRPC did not share one business listener: %v, %s", running.businessAddrs, running.grpcAddr)
				}
				assertHttpResponse(t, running.grpcAddr, "/api/", http.StatusOK, "mixed")
			}
			select {
			case <-polled:
			case <-time.After(serviceTestTimeout):
				t.Fatal("service did not confirm its registration")
			}
			if registerCount.Load() != 1 {
				t.Fatalf("registration count = %d, want one shared business registration", registerCount.Load())
			}
		})
	}
}

func TestConsulSkipsMissingIdentityWithoutStoppingService(t *testing.T) {
	for _, missing := range []string{"business listener", "service name"} {
		t.Run(missing, func(t *testing.T) {
			clearPodEnvironment(t)
			setConsulTestServiceName(t, "orders")
			var requests atomic.Int32
			client := testConsulClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
			}))
			opts := []ServiceOption{WithConsulRegister(consulutils.RegistrationConfig{Client: client})}
			if missing == "service name" {
				share.SetServiceName("")
				opts = append(opts, WithServiceAddress("127.0.0.1:0"), WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.NotFoundHandler()}))
			}
			running := startTestService(t, opts...)
			assertHttpResponse(t, running.httpAddr, "/ready", http.StatusOK, "ok")
			running.service.cancel()
			if err := running.wait(t); err != nil {
				t.Fatal(err)
			}
			if requests.Load() != 0 {
				t.Fatal("service contacted Consul without a business listener or service name")
			}
		})
	}
}

func TestConsulOptionAcceptsOnlyOneOptionalConfiguration(t *testing.T) {
	service := NewCores(WithConsulRegister())
	defer service.cancel()
	if service.consulConfig == nil {
		t.Fatal("default option did not enable Consul registration")
	}
	WithConsulRegister(consulutils.RegistrationConfig{}, consulutils.RegistrationConfig{})(service)
	if service.consulConfig != nil {
		t.Fatal("multiple configurations should disable registration")
	}
}
