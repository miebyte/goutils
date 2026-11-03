package cores

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/miebyte/goutils/consulutils"
	"github.com/miebyte/goutils/discover"
)

type fixedDiscoverySource struct {
	address string
}

func (s fixedDiscoverySource) Discover(ctx context.Context, _ string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []string{s.address}, nil
}

func resetDefaultDiscovery(t *testing.T) {
	t.Helper()
	discover.SetDefaultResolver(nil)
	t.Cleanup(func() { discover.SetDefaultResolver(nil) })
}

func TestConsulRegisterInitializesDiscoveryWithConfiguredClient(t *testing.T) {
	resetDefaultDiscovery(t)
	var fallbackRequests atomic.Int32
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackRequests.Add(1)
		_ = json.NewEncoder(w).Encode([]*consulapi.ServiceEntry{{
			Service: &consulapi.AgentService{Address: "default.invalid", Port: 9999},
		}})
	}))
	t.Cleanup(fallback.Close)
	t.Setenv("CONSUL_HTTP_ADDR", fallback.URL)
	t.Setenv("CONSUL_HTTP_TOKEN", "")
	t.Setenv("CONSUL_NAMESPACE", "")

	var requests atomic.Int32
	client := testConsulClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/v1/health/service/orders" || r.URL.Query().Get("passing") != "1" {
			t.Errorf("unexpected discovery request: %s %s", r.Method, r.URL)
		}
		_ = json.NewEncoder(w).Encode([]*consulapi.ServiceEntry{{
			Service: &consulapi.AgentService{Address: "127.0.0.1", Port: 8080},
		}})
	}))
	service := NewCores(WithConsulRegister(consulutils.RegistrationConfig{Client: client}))
	t.Cleanup(service.cancel)
	if requests.Load() != 0 || fallbackRequests.Load() != 0 {
		t.Fatal("constructing a service contacted Consul")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	address, err := discover.Resolve(ctx, "orders")
	if err != nil {
		t.Fatal(err)
	}
	if address != "127.0.0.1:8080" || requests.Load() != 1 || fallbackRequests.Load() != 0 {
		t.Fatalf("discovery = %q, configured requests = %d, default requests = %d; want configured Consul only", address, requests.Load(), fallbackRequests.Load())
	}
}

func TestConsulRegisterPreservesExplicitDiscoverySource(t *testing.T) {
	resetDefaultDiscovery(t)
	discover.SetDefaultResolver(fixedDiscoverySource{address: "local:8080"})
	var requests atomic.Int32
	client := testConsulClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_ = json.NewEncoder(w).Encode([]*consulapi.ServiceEntry{{
			Service: &consulapi.AgentService{Address: "consul", Port: 9090},
		}})
	}))
	service := NewCores(WithConsulRegister(consulutils.RegistrationConfig{Client: client}))
	t.Cleanup(service.cancel)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	address, err := discover.Resolve(ctx, "orders")
	if err != nil {
		t.Fatal(err)
	}
	if address != "local:8080" || requests.Load() != 0 {
		t.Fatalf("discovery = %q, Consul requests = %d; want the explicitly configured source", address, requests.Load())
	}
}

func TestConsulDiscoveryDefaultCanBeOverriddenAfterConstruction(t *testing.T) {
	resetDefaultDiscovery(t)
	var requests atomic.Int32
	client := testConsulClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	service := NewCores(WithConsulRegister(consulutils.RegistrationConfig{Client: client}))
	t.Cleanup(service.cancel)
	discover.SetDefaultResolver(fixedDiscoverySource{address: "override:8080"})

	address, err := discover.Resolve(context.Background(), "orders")
	if err != nil {
		t.Fatal(err)
	}
	if address != "override:8080" || requests.Load() != 0 {
		t.Fatalf("discovery = %q, Consul requests = %d; want the later explicit source", address, requests.Load())
	}
}
