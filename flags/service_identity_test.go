package flags

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/miebyte/goutils/share"
	"github.com/spf13/pflag"
)

func TestParsePreservesExplicitServiceIdentity(t *testing.T) {
	mode := os.Getenv("GOUTILS_FLAGS_IDENTITY_TEST")
	if mode == "" {
		// Parse 使用全局 FlagSet，每个场景通过独立进程执行一次完整启动解析。
		for _, mode := range []string{"explicit", "config"} {
			t.Run(mode, func(t *testing.T) {
				cmd := exec.Command(os.Args[0], "-test.run=^TestParsePreservesExplicitServiceIdentity$")
				cmd.Env = append(os.Environ(), "GOUTILS_FLAGS_IDENTITY_TEST="+mode)
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
	// 显式服务名中的冒号也不由 flags 拆分，旧环境变量的兼容解析由 share 自己负责。
	const name = "orders:explicit"
	share.SetServiceName(name)
	share.SetTag("explicit-tag")
	probe := String("probe", "default", "")
	contents, wantTag := `{"probe":"loaded"}`, "explicit-tag"
	if mode == "config" {
		contents = `{"probe":"loaded","serviceName":"config-orders:config-tag","serviceTag":"config-tag","debug":true}`
		wantTag = "config-tag"
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"service-identity", "--configFile=" + path}
	Parse()

	if got := share.GetServiceName(); got != name {
		t.Fatalf("service identity after Parse = %q, want %q", got, name)
	}
	if pflag.Lookup("serviceName") != nil || pflag.CommandLine.ShorthandLookup("s") != nil {
		t.Fatal("Parse registered a built-in serviceName or -s flag")
	}
	if probe() != "loaded" || share.Tag() != wantTag {
		t.Fatalf("configuration was not parsed: probe=%q, tag=%q", probe(), share.Tag())
	}
	if mode == "config" && !share.Debug() {
		t.Fatal("removing service identity parsing changed the debug configuration behavior")
	}
}
