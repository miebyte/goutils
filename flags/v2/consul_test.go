package flags

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	consulapi "github.com/hashicorp/consul/api"
)

func TestNewConsulProviderPath(t *testing.T) {
	for _, tc := range []struct {
		prefix string
		key    string
		want   string
	}{
		{"", "app", "app"},
		{"", "/app", "app"},
		{"/config/", "/app", "config/app"},
		{"config/service", "app", "config/service/app"},
	} {
		t.Run(tc.prefix+"/"+tc.key, func(t *testing.T) {
			source := NewConsulProvider(&consulapi.Config{}, tc.prefix)
			t.Cleanup(func() { _ = source.Close() })
			if got := source.Path(tc.key); got != tc.want {
				t.Fatalf("Path(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestWithRemoteConfigDefaultsToConsul(t *testing.T) {
	raw := `{"limit":3}`
	config, requests := newFlagsConsulTestConfig(t, "app", &raw)
	for _, name := range []string{
		"CONSUL_HTTP_TOKEN", "CONSUL_HTTP_TOKEN_FILE", "CONSUL_HTTP_AUTH",
		"CONSUL_CACERT", "CONSUL_CAPATH", "CONSUL_CLIENT_CERT", "CONSUL_CLIENT_KEY",
		"CONSUL_TLS_SERVER_NAME", "CONSUL_DATACENTER", "CONSUL_NAMESPACE", "CONSUL_PARTITION",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("CONSUL_HTTP_ADDR", config.Address)
	t.Setenv("CONSUL_HTTP_SSL", "false")

	source, err := getConfigProvider(initOption(WithRemoteConfig()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	if requests.Load() != 0 {
		t.Fatal("remote provider accessed Consul before reading configuration")
	}
	value, err := source.ReadConfig(context.Background(), source.Path("app"))
	if err != nil || string(value) != raw {
		t.Fatalf("default Consul config = %s, %v", value, err)
	}
	if requests.Load() != 1 {
		t.Fatalf("Consul requests = %d, want 1", requests.Load())
	}
}

func TestWithRemoteConsulProvider(t *testing.T) {
	raw := `{"limit":4}`
	config, requests := newFlagsConsulTestConfig(t, "config/app", &raw)
	source, err := getConfigProvider(initOption(WithRemoteConfig(WithRemoteConsulProvider(config, "/config/"))))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	if requests.Load() != 0 {
		t.Fatal("remote provider accessed Consul before reading configuration")
	}
	value, err := source.ReadConfig(context.Background(), source.Path("app"))
	if err != nil || string(value) != raw {
		t.Fatalf("explicit Consul config = %s, %v", value, err)
	}
	if requests.Load() != 1 {
		t.Fatalf("Consul requests = %d, want 1", requests.Load())
	}
}

func TestConsulExplicitLocalConfigTakesPrecedence(t *testing.T) {
	raw := `{"limit":9}`
	config, requests := newFlagsConsulTestConfig(t, "config/app", &raw)
	localPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(localPath, []byte(`{"app":{"limit":2}}`), 0600); err != nil {
		t.Fatal(err)
	}
	opt := initOption(WithRemoteConfig(WithRemoteConsulProvider(config, "config")))
	opt.ConfigPath = localPath
	source, err := getConfigProvider(opt)
	if err != nil {
		t.Fatal(err)
	}
	owner := newConsulTestSuperFlags(t)
	owner.SetConfigProvider(source)
	parser := Struct("app", &consulTestStruct{Limit: 1}, "", WithConfigPath[*consulTestStruct]("custom/app"))
	if err := owner.ReadConfig(); err != nil {
		t.Fatal(err)
	}
	value, err := parser()
	if err != nil || value.Load().Limit != 2 {
		t.Fatalf("local Struct config = %v, %v", value, err)
	}
	if requests.Load() != 0 {
		t.Fatalf("local configuration made %d Consul requests", requests.Load())
	}
}

func TestConsulStructInitialization(t *testing.T) {
	valid := `{"limit":5}`
	invalid := `{"limit":`
	for _, tc := range []struct {
		name    string
		raw     *string
		want    int
		wantErr string
	}{
		{"valid JSON", &valid, 5, ""},
		{"missing config uses default", nil, 1, ""},
		{"invalid JSON", &invalid, 0, "invalid JSON"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, _ := newFlagsConsulTestConfig(t, "config/app", tc.raw)
			owner := newConsulTestSuperFlags(t)
			parser := Struct("app", &consulTestStruct{Limit: 1}, "")
			owner.SetConfigProvider(NewConsulProvider(config, "config"))
			err := owner.ReadConfig()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("ReadConfig() = %v, want error containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			value, err := parser()
			if err != nil || value.Load().Limit != tc.want {
				t.Fatalf("Struct config = %v, %v; want limit %d", value, err, tc.want)
			}
		})
	}
}

func TestConsulStructConfigPathUsesCompleteKey(t *testing.T) {
	raw := `{"limit":7}`
	config, _ := newFlagsConsulTestConfig(t, "custom/app", &raw)
	owner := newConsulTestSuperFlags(t)
	parser := Struct("app", &consulTestStruct{Limit: 1}, "", WithConfigPath[*consulTestStruct]("custom/app"))
	owner.SetConfigProvider(NewConsulProvider(config, "config"))
	if err := owner.ReadConfig(); err != nil {
		t.Fatal(err)
	}
	value, err := parser()
	if err != nil || value.Load().Limit != 7 {
		t.Fatalf("Struct config with complete key = %v, %v", value, err)
	}
}

type consulTestStruct struct {
	Limit int `json:"limit"`
}

func newConsulTestSuperFlags(t *testing.T) *SuperFlags {
	t.Helper()
	previous := sf
	owner := New()
	sf = owner
	t.Cleanup(func() {
		_ = owner.Close()
		sf = previous
	})
	return owner
}

func newFlagsConsulTestConfig(t *testing.T, key string, raw *string) (*consulapi.Config, *atomic.Int32) {
	t.Helper()
	requests := new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/v1/kv/"+key {
			t.Errorf("Consul request = %s %s, want GET /v1/kv/%s", r.Method, r.URL.Path, key)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Consul-Index", "1")
		if raw == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]*consulapi.KVPair{{Key: key, Value: []byte(*raw), CreateIndex: 1, ModifyIndex: 1}}); err != nil {
			t.Errorf("encode Consul response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return &consulapi.Config{
		Address:    strings.TrimPrefix(server.URL, "http://"),
		Scheme:     "http",
		HttpClient: server.Client(),
	}, requests
}
