package discover_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/miebyte/goutils/discover"
	"github.com/miebyte/goutils/flags/v2"
	"github.com/miebyte/goutils/share"
	"github.com/miebyte/goutils/snail"
	"github.com/stretchr/testify/require"
)

func TestParseInitializesDiscoveryBeforeUserCallbacks(t *testing.T) {
	mode := os.Getenv("GOUTILS_DISCOVER_PARSE_TEST")
	if mode == "" {
		// Parse、snail 和共享 Consul Client 都有进程级状态，各场景独立启动。
		for _, mode := range []string{"consul", "local", "explicit"} {
			t.Run(mode, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestParseInitializesDiscoveryBeforeUserCallbacks$")
				cmd.Env = append(os.Environ(), "GOUTILS_DISCOVER_PARSE_TEST="+mode)
				output, err := cmd.CombinedOutput()
				require.NoError(t, err, "%s", output)
			})
		}
		return
	}

	t.Chdir(t.TempDir())
	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		if strings.HasPrefix(key, "CONSUL_") {
			t.Setenv(key, "")
		}
	}
	var defaultRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defaultRequests.Add(1)
		if r.URL.Path != "/v1/health/service/orders" || r.URL.Query().Get("passing") != "1" {
			t.Errorf("unexpected default Consul request: %s", r.URL)
		}
		_, _ = w.Write([]byte(`[{"Service":{"Address":"orders.consul.svc","Port":8080}}]`))
	}))
	defer server.Close()
	t.Setenv("CONSUL_HTTP_ADDR", server.URL)
	configPath := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{}`), 0600))
	os.Args = []string{"discover-parse", "--configFile=" + configPath}
	share.SetServiceName("discover-parse-test")
	want := "orders.consul.svc:8080"
	var explicitRequests atomic.Int32
	if mode == "local" {
		os.Args = append(os.Args, "--use-local-discovery")
		require.NoError(t, os.WriteFile("services.json", []byte(`{"orders":["127.0.0.1:8080"],"disabled":[]}`), 0600))
		want = "127.0.0.1:8080"
	} else if mode == "explicit" {
		// 显式源优先，即使 localflag 打开且工作目录内没有 services.json。
		os.Args = append(os.Args, "--use-local-discovery")
		explicitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			explicitRequests.Add(1)
			if r.Header.Get("X-Consul-Token") != "explicit-token" {
				t.Errorf("explicit Consul token = %q", r.Header.Get("X-Consul-Token"))
			}
			_, _ = w.Write([]byte(`[{"Service":{"Address":"orders.explicit.svc","Port":9090}}]`))
		}))
		defer explicitServer.Close()
		config := api.DefaultConfig()
		config.Address = explicitServer.URL
		config.Token = "explicit-token"
		client, err := api.NewClient(config)
		require.NoError(t, err)
		discover.SetDefaultResolver(discover.NewConsulDiscovery(client))
		want = "orders.explicit.svc:9090"
	}

	var address string
	called := false
	snail.RegisterObject("resolve-orders-before-cores-construction", func() error {
		called = true
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var err error
		address, err = discover.Resolve(ctx, "orders")
		return err
	})
	flags.Parse()
	defer flags.Close()
	require.True(t, called)
	require.Equal(t, want, address)
	if mode == "consul" {
		require.EqualValues(t, 1, defaultRequests.Load())
	} else {
		require.Zero(t, defaultRequests.Load())
	}
	if mode == "explicit" {
		require.EqualValues(t, 1, explicitRequests.Load())
	} else if mode == "local" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, err := discover.Resolve(ctx, "disabled")
		require.ErrorIs(t, err, discover.ErrNoAvailableService)
		require.Zero(t, defaultRequests.Load(), "empty local services must not fall back to Consul")
	}
}
