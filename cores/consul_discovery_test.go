package cores

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/miebyte/goutils/consulutils"
	"github.com/miebyte/goutils/discover"
)

func resetDefaultDiscovery(t *testing.T) {
	t.Helper()
	discover.SetDefaultResolver(nil)
	t.Cleanup(func() { discover.SetDefaultResolver(nil) })
}

func assertDefaultDiscoveryAddress(t *testing.T, want string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	address, err := discover.Resolve(ctx, "orders")
	if err != nil {
		t.Fatal(err)
	}
	if address != want {
		t.Fatalf("discovery = %q, want %q", address, want)
	}
}

func TestConsulRegisterPreservesDiscoveryConfiguredBeforeConstruction(t *testing.T) {
	resetDefaultDiscovery(t)
	var discoveryRequests atomic.Int32
	client := testConsulClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discoveryRequests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/v1/health/service/orders" || r.URL.Query().Get("passing") != "1" {
			t.Errorf("unexpected discovery request: %s %s", r.Method, r.URL)
		}
		_ = json.NewEncoder(w).Encode([]*consulapi.ServiceEntry{{
			Service: &consulapi.AgentService{Address: "127.0.0.1", Port: 8080},
		}})
	}))
	discover.SetDefaultResolver(discover.NewConsulDiscovery(client))
	assertDefaultDiscoveryAddress(t, "127.0.0.1:8080")

	var registrationRequests atomic.Int32
	registrationClient := testConsulClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		registrationRequests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	service := NewCores(WithConsulRegister(consulutils.RegistrationConfig{Client: registrationClient}))
	t.Cleanup(service.cancel)
	assertDefaultDiscoveryAddress(t, "127.0.0.1:8080")
	if discoveryRequests.Load() != 2 || registrationRequests.Load() != 0 {
		t.Fatalf("discovery requests = %d, registration requests = %d; want the original discovery Client before and after construction", discoveryRequests.Load(), registrationRequests.Load())
	}
}

func TestConsulRegisterPreservesLocalDiscovery(t *testing.T) {
	resetDefaultDiscovery(t)
	path := filepath.Join(t.TempDir(), "services.json")
	if err := os.WriteFile(path, []byte(`{"orders":["127.0.0.1:8080"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	local, err := discover.NewLocal(path)
	if err != nil {
		t.Fatal(err)
	}
	discover.SetDefaultResolver(local)
	assertDefaultDiscoveryAddress(t, "127.0.0.1:8080")

	var requests atomic.Int32
	client := testConsulClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	service := NewCores(WithConsulRegister(consulutils.RegistrationConfig{Client: client}))
	t.Cleanup(service.cancel)
	assertDefaultDiscoveryAddress(t, "127.0.0.1:8080")
	if requests.Load() != 0 {
		t.Fatalf("Consul requests = %d, want the local source before and after construction", requests.Load())
	}
}

func TestConsulRegisterDoesNotInitializeDiscovery(t *testing.T) {
	resetDefaultDiscovery(t)
	client := testConsulClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	service := NewCores(WithConsulRegister(consulutils.RegistrationConfig{Client: client}))
	t.Cleanup(service.cancel)

	defer func() {
		if recover() == nil {
			t.Fatal("constructing a service initialized discovery without flags.Parse or an explicit source")
		}
	}()
	_, _ = discover.Resolve(context.Background(), "orders")
}
