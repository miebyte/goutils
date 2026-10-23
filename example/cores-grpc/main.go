package main

import (
	"net/http"

	"github.com/miebyte/goutils/cores"
	"github.com/miebyte/goutils/flags"
	"github.com/miebyte/goutils/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

var (
	address         = flags.String("address", ":9000", "HTTP 与 gRPC 共用的业务监听地址")
	httpEnabled     = flags.Bool("http", false, "同时在业务端口启用 HTTP 示例接口")
	internalAddress = flags.String("internal-address", ":6060", "管理 HTTP 监听地址")
	pprofEnabled    = flags.Bool("pprof", false, "在管理端口启用 pprof")
)

func main() {
	flags.Parse()

	grpcServer := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	// 注册 reflection，便于通过 grpcurl 调用示例中的健康服务。
	reflection.Register(grpcServer)

	opts := []cores.ServiceOption{
		// 服务模式始终启动管理 HTTP，覆盖地址在构造阶段设置。
		cores.WithInternalHTTPAddress(internalAddress()),
		cores.WithServiceAddress(address()),
		cores.WithGrpcServer(grpcServer),
	}
	if httpEnabled() {
		// HTTP 和 gRPC 使用统一业务地址，框架通过 cmux 按连接分流。
		// 共享端口使用明文 HTTP/gRPC；需要 TLS 时应在入口代理终止 TLS。
		opts = append(opts, cores.WithHttpHandler(cores.HttpHandler{
			ApiPrefix: "/hello",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("hello\n"))
			}),
		}))
	}
	if pprofEnabled() {
		opts = append(opts, cores.WithPprof())
	}

	srv := cores.NewCores(opts...)
	logging.PanicError(cores.Start(srv))
}
