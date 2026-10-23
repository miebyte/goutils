package flags

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/miebyte/goutils/share"
	"github.com/spf13/pflag"
)

func TestParsePreservesExplicitServiceIdentity(t *testing.T) {
	mode := os.Getenv("GOUTILS_FLAGS_V2_IDENTITY_TEST")
	if mode == "" {
		// 隔离 Parse 的全局声明和状态，两个场景均走真实参数解析及配置读取流程。
		for _, mode := range []string{"local", "consul"} {
			t.Run(mode, func(t *testing.T) {
				cmd := exec.Command(os.Args[0], "-test.run=^TestParsePreservesExplicitServiceIdentity$")
				cmd.Env = append(os.Environ(), "GOUTILS_FLAGS_V2_IDENTITY_TEST="+mode)
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("Parse subprocess: %v\n%s", err, output)
				}
			})
		}
		return
	}

	sf = New()
	requiredFlags = nil
	pflag.CommandLine = pflag.NewFlagSet("service-identity", pflag.ContinueOnError)
	share.SetServiceName("orders")
	os.Args = []string{"service-identity"}
	defer Close()

	if mode == "local" {
		probe := String("probe", "default", "")
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(`{"probe":"loaded","serviceName":"config-orders","debug":true}`), 0600); err != nil {
			t.Fatal(err)
		}
		os.Args = append(os.Args, "--configFile="+path)
		Parse()
		if probe() != "loaded" || !share.Debug() {
			t.Fatalf("local configuration was not parsed: probe=%q, debug=%v", probe(), share.Debug())
		}
	} else {
		type serviceConfig struct {
			ServiceName string `json:"serviceName"`
			Limit       int    `json:"limit"`
		}
		parser := Struct("app", (*serviceConfig)(nil), "")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 路径中的服务名来自 Parse 前设置的 share 身份，而非命令行或配置里的字段。
			if r.URL.Path != "/v1/kv/configs/orders/app" {
				t.Errorf("Consul path = %q", r.URL.Path)
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode([]*consulapi.KVPair{{
				Key: "configs/orders/app", Value: []byte(`{"serviceName":"config-orders","limit":7}`),
			}})
		}))
		defer server.Close()
		config := consulapi.DefaultConfig()
		config.Address = server.URL
		client, err := consulapi.NewClient(config)
		if err != nil {
			t.Fatal(err)
		}
		Parse(WithRemoteConfig(WithRemoteConsulProvider(client)))
		value, err := parser()
		if err != nil {
			t.Fatal(err)
		}
		if got := value.Load(); got.Limit != 7 || got.ServiceName != "config-orders" {
			t.Fatalf("Consul configuration was not parsed: %+v", got)
		}
	}
	if got := share.GetServiceName(); got != "orders" {
		t.Fatalf("service identity after Parse = %q, want orders", got)
	}
	if pflag.Lookup("serviceName") != nil || pflag.CommandLine.ShorthandLookup("s") != nil {
		t.Fatal("Parse registered a built-in serviceName or -s flag")
	}
}
