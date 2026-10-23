package main

import (
	"fmt"
	"net/http"

	"github.com/miebyte/goutils/cores"
	"github.com/miebyte/goutils/flags/v2"
	"github.com/miebyte/goutils/logging"
)

type AppConfig struct {
	Message string `json:"message"`
}

var (
	port            = flags.Int("port", 8080, "业务 HTTP 端口")
	internalAddress = flags.String("internal-address", ":6060", "管理 HTTP 监听地址")
	registerEnabled = flags.Bool("consul-register", false, "启用 Consul 服务注册")
	pprofEnabled    = flags.Bool("pprof", false, "在管理端口启用 pprof")
	appConfig       = flags.Struct("app", &AppConfig{Message: "hello from orders"}, "应用配置")
)

func main() {
	// 配置读取、配置监听和服务注册默认共用 consulutils.DefaultClient()。
	// 远程配置初始化失败仍会终止启动，与注册失败后台重试的策略独立。
	// 显式传入 --configFile 时改用本地文件，便于单独验证注册失败的恢复行为。
	flags.Parse(
		flags.WithRemoteConfig(flags.WithServiceName("orders")),
		flags.WithConfigWatch(),
	)
	defer flags.Close()

	config, err := appConfig()
	logging.PanicError(err)

	opts := []cores.ServiceOption{
		cores.WithInternalHTTPAddress(internalAddress()),
		cores.WithServiceAddress(fmt.Sprintf(":%d", port())),
		cores.WithHttpHandler(cores.HttpHandler{
			ApiPrefix: "/",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Load 返回当前只读快照，不修改其中的字段。
				_, _ = fmt.Fprintln(w, config.Load().Message)
			}),
		}),
	}
	if pprofEnabled() {
		opts = append(opts, cores.WithPprof())
	}
	if registerEnabled() {
		// 注册身份由框架统一取得：服务名沿用 flags，端口使用业务实际监听端口。
		opts = append(opts, cores.WithConsulRegister())
	}

	// Start 始终开启管理 HTTP；Consul 检查自动指向该实例的 /ready。
	logging.PanicError(cores.Start(cores.NewCores(opts...)))
}
