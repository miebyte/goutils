package flags

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/miebyte/goutils/internal/share"
)

func TestConsulProviderSharesInjectedClientForReadAndWatch(t *testing.T) {
	previousName := share.ServiceName
	t.Cleanup(func() { share.ServiceName = previousName })
	share.SetServiceName("before-parse")
	changed := make(chan struct{})
	var reads, watches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Consul-Token") != "injected-token" {
			t.Errorf("Consul token = %q", r.Header.Get("X-Consul-Token"))
		}
		if r.URL.Path == "/v1/agent/self" {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		if r.URL.Path != "/v1/kv/configs/parsed-service/app" {
			t.Errorf("Consul request path = %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		raw, index := `{"limit":1}`, "1"
		reads.Add(1)
		if r.URL.Query().Get("index") != "" {
			watches.Add(1)
		}
		switch r.URL.Query().Get("index") {
		case "1":
			select {
			case <-changed:
				raw, index = `{"limit":2}`, "2"
			case <-r.Context().Done():
				return
			}
		case "2":
			<-r.Context().Done()
			return
		}
		w.Header().Set("X-Consul-Index", index)
		_ = json.NewEncoder(w).Encode([]*consulapi.KVPair{{Key: "configs/parsed-service/app", Value: []byte(raw)}})
	}))
	t.Cleanup(server.Close)
	config := consulapi.DefaultConfig()
	config.Address, config.Token = server.URL, "injected-token"
	client, err := consulapi.NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	var options RemoteConfigOption
	WithRemoteConsulProvider(client)(&options)
	source := options.provider
	t.Cleanup(func() { _ = source.Close() })

	// 选项先创建，服务名后解析；首次使用时才拼出最终的配置前缀。
	share.SetServiceName("parsed-service")
	path := source.Path("app")
	if path != "/configs/parsed-service/app" {
		t.Fatalf("Path = %q", path)
	}
	raw, err := source.ReadConfig(context.Background(), path)
	if err != nil || string(raw) != `{"limit":1}` {
		t.Fatalf("ReadConfig = %s, %v", raw, err)
	}
	events, err := source.WatchConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		t.Fatalf("WatchConfig published initial state: %+v", event)
	default:
	}
	close(changed)
	select {
	case event := <-events:
		if event.Err != nil || string(event.Config) != `{"limit":2}` || event.Path != path {
			t.Fatalf("WatchConfig update = %+v", event)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for Consul watch update")
	}
	if reads.Load() < 3 || watches.Load() < 1 {
		t.Fatalf("Consul requests: reads = %d, watches = %d", reads.Load(), watches.Load())
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("watch event stream remained open after Close")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not stop Consul watch")
	}
	// 配置关闭后，同一 client 仍可供注册器调用 Agent API。
	if _, err := client.Agent().Self(); err != nil {
		t.Fatalf("shared client stopped working after provider Close: %v", err)
	}
}

func TestConsulProviderRejectsInvalidClientArguments(t *testing.T) {
	for _, clients := range [][]*consulapi.Client{{nil}, {nil, nil}} {
		source := NewConsulProvider(clients...)
		if _, err := source.ReadConfig(context.Background(), "app"); err == nil {
			t.Fatalf("NewConsulProvider(%v) accepted invalid clients", clients)
		}
		_ = source.Close()
	}
}
