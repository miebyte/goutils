package share

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/miebyte/goutils/buildinfo"
)

func TestServiceNameSources(t *testing.T) {
	if os.Getenv("GOUTILS_SHARE_ENV_TEST") == "1" {
		// 模拟构建注入，环境变量则在子进程启动前设置，由包初始化读取。
		buildinfo.ServiceName = "built-orders"
		buildinfo.Version = "built-tag"
		if got, want := GetServiceName(), os.Getenv("GOUTILS_SHARE_EXPECT_NAME"); got != want {
			t.Fatalf("service name = %q, want %q", got, want)
		}
		if got, want := Tag(), os.Getenv("GOUTILS_SHARE_EXPECT_TAG"); got != want {
			t.Fatalf("service tag = %q, want %q", got, want)
		}

		SetServiceName("explicit-orders")
		if got := GetServiceName(); got != "explicit-orders" {
			t.Fatalf("explicit service name = %q", got)
		}
		// 显式设置空名称也不应再次回退，避免配置读取和注册看到不同身份。
		SetServiceName("")
		if got := GetServiceName(); got != "" {
			t.Fatalf("explicit empty name fell back to %q", got)
		}
		return
	}

	for _, test := range []struct {
		name, value, wantName, wantTag string
	}{
		{name: "build default", wantName: "built-orders", wantTag: "built-tag"},
		{name: "environment", value: "env-orders", wantName: "env-orders", wantTag: "built-tag"},
		{name: "environment with tag", value: "env-orders:v2", wantName: "env-orders", wantTag: "v2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestServiceNameSources$")
			for _, env := range os.Environ() {
				key, _, _ := strings.Cut(env, "=")
				if key != ServiceNameKey && !strings.HasPrefix(key, "GOUTILS_SHARE_") {
					cmd.Env = append(cmd.Env, env)
				}
			}
			cmd.Env = append(cmd.Env,
				"GOUTILS_SHARE_ENV_TEST=1",
				ServiceNameKey+"="+test.value,
				"GOUTILS_SHARE_EXPECT_NAME="+test.wantName,
				"GOUTILS_SHARE_EXPECT_TAG="+test.wantTag,
			)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("share initialization failed: %v\n%s", err, output)
			}
		})
	}
}
