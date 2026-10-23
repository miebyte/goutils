package cores

import (
	"context"
	"net"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

type grpcServer struct {
	server   *grpc.Server
	listener net.Listener
}

// WithGrpcServer 在 Start 服务模式中运行原生 gRPC Server。
// 业务地址统一通过 WithServiceAddress 指定，和 HTTP 共用时由 cmux 分流；管理端口始终独立。
// 服务注册和配置需在传入前完成，cores 负责监听器和优雅退出。
// 传入的 Server 不应已在其他位置启动，Run 脚本模式不支持业务监听器。
func WithGrpcServer(server *grpc.Server) ServiceOption {
	return func(cs *CoresService) {
		if server == nil {
			cs.configErr = errors.New("grpc server must not be nil")
			return
		}

		cs.grpc = &grpcServer{server: server}
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
			if c.stopping.Load() && (errors.Is(err, grpc.ErrServerStopped) || isBusinessListenerClosed(err)) {
				return nil
			}
			return err
		},
	}
}
