package cores

import (
	"context"
	"net"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

type grpcServer struct {
	addr     string
	server   *grpc.Server
	listener net.Listener
}

// WithGrpcServer runs server on a separate TCP address when Run or Start is called.
// Register services and configure server before passing it to cores, which owns
// its listener and shutdown. The server must not already be serving.
func WithGrpcServer(addr string, server *grpc.Server) ServiceOption {
	return func(cs *CoresService) {
		cs.grpc = &grpcServer{addr: addr, server: server}
	}
}

func (c *CoresService) listenGrpc() mountFn {
	return mountFn{
		name: "GrpcListener",
		fn: func(ctx context.Context) error {
			result := make(chan error, 1)
			go func() {
				result <- c.grpc.server.Serve(c.grpc.listener)
			}()

			var err error
			select {
			case err = <-result:
			case <-ctx.Done():
				// Shutdown has finished or timed out. Native gRPC may still be
				// waiting for a handler that ignores cancellation.
				return nil
			}
			if c.stopping.Load() && (errors.Is(err, grpc.ErrServerStopped) || errors.Is(err, net.ErrClosed)) {
				return nil
			}
			return err
		},
	}
}
