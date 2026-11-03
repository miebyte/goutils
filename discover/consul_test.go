package discover

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestConsulDiscoveryUsesConfiguredClient(t *testing.T) {
	var defaultRequests atomic.Int32
	defaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defaultRequests.Add(1)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer defaultServer.Close()
	t.Setenv("CONSUL_HTTP_ADDR", defaultServer.URL)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/v1/health/service/orders" || r.URL.Query().Get("passing") != "1" || r.Header.Get("X-Consul-Token") != "custom-token" {
			t.Errorf("unexpected custom Consul request: %s %v", r.URL, r.Header)
		}
		_, _ = w.Write([]byte(`[{"Service":{"Address":"orders.test.svc","Port":8080}}]`))
	}))
	defer server.Close()
	config := api.DefaultConfig()
	config.Address = server.URL
	config.Token = "custom-token"
	client, err := api.NewClient(config)
	require.NoError(t, err)

	addresses, err := NewConsulDiscovery(client).Discover(context.Background(), "orders")
	require.NoError(t, err)
	require.Equal(t, []string{"orders.test.svc:8080"}, addresses)
	require.EqualValues(t, 1, requests.Load())
	require.Zero(t, defaultRequests.Load(), "custom discovery must not query the default Consul")
}
