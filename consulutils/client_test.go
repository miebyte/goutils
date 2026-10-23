package consulutils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	consulapi "github.com/hashicorp/consul/api"
)

func TestDefaultClientSharesClientWithoutConnecting(t *testing.T) {
	if os.Getenv("GOUTILS_CONSUL_CLIENT_TEST") != "shared" {
		runDefaultClientTest(t, "shared")
		return
	}
	// 不依赖开发机上的 Consul 配置，首次构造只应创建连接池。
	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		if strings.HasPrefix(key, "CONSUL_") {
			t.Setenv(key, "")
		}
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("X-Consul-Token") != "shared-token" {
			t.Errorf("Consul token = %q", r.Header.Get("X-Consul-Token"))
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	t.Setenv("CONSUL_HTTP_ADDR", server.URL)
	t.Setenv("CONSUL_HTTP_TOKEN", "shared-token")

	const callers = 32
	clients := make([]*consulapi.Client, callers)
	errors := make([]error, callers)
	var calls sync.WaitGroup
	for i := range callers {
		calls.Go(func() { clients[i], errors[i] = DefaultClient() })
	}
	calls.Wait()
	for i := range callers {
		if errors[i] != nil || clients[i] == nil || clients[i] != clients[0] {
			t.Fatalf("DefaultClient[%d] = %p, %v; want %p", i, clients[i], errors[i], clients[0])
		}
	}
	if requests.Load() != 0 {
		t.Fatal("constructing the client made a network request")
	}
	if _, _, err := clients[0].KV().Get("config", (&consulapi.QueryOptions{}).WithContext(context.Background())); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 {
		t.Fatalf("KV requests = %d, want 1", requests.Load())
	}

	// 首次创建后，修改环境变量不会让配置读取和服务注册分裂为两个 client。
	t.Setenv("CONSUL_HTTP_ADDR", "unsupported://consul")
	if client, err := DefaultClient(); err != nil || client != clients[0] {
		t.Fatalf("DefaultClient after env change = %p, %v", client, err)
	}
}

func TestDefaultClientCachesConfigurationError(t *testing.T) {
	// 使用独立进程验证首次构造失败，不重置进程级单例而影响其他测试。
	if os.Getenv("GOUTILS_CONSUL_CLIENT_TEST") == "invalid" {
		first, firstErr := DefaultClient()
		second, secondErr := DefaultClient()
		if first != nil || second != nil || firstErr == nil || firstErr != secondErr {
			t.Fatalf("DefaultClient errors = %p, %v; %p, %v", first, firstErr, second, secondErr)
		}
		return
	}
	runDefaultClientTest(t, "invalid")
}

func runDefaultClientTest(t *testing.T, mode string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "CONSUL_") && !strings.HasPrefix(env, "GOUTILS_CONSUL_CLIENT_TEST=") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "GOUTILS_CONSUL_CLIENT_TEST="+mode)
	if mode == "invalid" {
		cmd.Env = append(cmd.Env, "CONSUL_HTTP_ADDR=unsupported://consul")
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("client error subprocess: %v\n%s", err, output)
	}
}
