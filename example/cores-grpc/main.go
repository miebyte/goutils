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
	grpcAddress     = flags.String("grpc-address", ":9000", "gRPC listen address")
	httpAddress     = flags.String("http-address", "", "optional business HTTP listen address")
	internalAddress = flags.String("internal-address", "", "optional internal HTTP listen address for health and pprof")
)

func main() {
	flags.Parse()

	grpcServer := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	// Enable reflection so grpcurl can discover the health service in this example.
	reflection.Register(grpcServer)

	opts := []cores.ServiceOption{
		cores.WithGrpcServer(grpcAddress(), grpcServer),
	}
	if httpAddress() != "" {
		opts = append(opts, cores.WithHttpHandler(httpAddress(), "/hello", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("hello\n"))
		})))
	}
	if internalAddress() != "" {
		opts = append(opts, cores.WithPprof())
	}

	srv := cores.NewCores(opts...)
	if internalAddress() == "" {
		logging.PanicError(cores.Run(srv))
		return
	}

	logging.PanicError(cores.Start(srv, internalAddress()))
}
