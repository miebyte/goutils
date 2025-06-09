package main

import (
	"github.com/miebyte/goutils/cores"
	"github.com/miebyte/goutils/example/grpc/examplepb"
	"github.com/miebyte/goutils/example/grpc/service"
	"github.com/miebyte/goutils/example/grpc/testpb"
	"github.com/miebyte/goutils/flags"
	"github.com/miebyte/goutils/logging"
	"google.golang.org/grpc"
)

func main() {
	flags.Parse()

	example := service.NewExampleService()
	test := service.NewTestService()

	srv := cores.NewCores(
		cores.WithGrpcUI(),
		cores.WithGrpcServer(func(s *grpc.Server) {
			examplepb.RegisterExampleHelloServiceServer(s, example)
			testpb.RegisterExampleHelloServiceServer(s, test)
		}),
	)
	logging.PanicError(cores.Start(srv, ":8080"))
}
