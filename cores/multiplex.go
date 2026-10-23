package cores

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/soheilhy/cmux"
)

// setupBusinessListener 只创建一个业务监听器，管理 HTTP 仍使用独立端口。
// 同时启用 HTTP 和 gRPC 时，先按 gRPC 的 HTTP/2 content-type 分流，再交给普通 HTTP。
func (c *CoresService) setupBusinessListener() error {
	if c.httpServer == nil && c.grpc == nil {
		return nil
	}

	if c.serviceAddress == "" {
		return errors.New("business listen address must not be empty; use WithServiceAddress")
	}

	// 启动一个端口监听
	listener, err := net.Listen("tcp", c.serviceAddress)
	if err != nil {
		return errors.Wrapf(err, "failed to start business listener at %s", c.serviceAddress)
	}
	c.businessListener = listener

	// 同时启用 HTTP 和 gRPC 时，使用端口复用。
	if c.httpServer != nil && c.grpc != nil {
		// 共享监听器只包装幂等关闭，连接由 cmux 和各自的 Server 管理。
		c.businessListener = &businessListener{Listener: listener}
		c.cmux = cmux.New(c.businessListener)
		// 未完成协议识别的连接交给读取超时处理，退出时也等待这部分识别任务结束。
		timeout := c.httpServerConfig.ReadHeaderTimeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		c.cmux.SetReadTimeout(timeout)
		c.grpc.listener = c.cmux.MatchWithWriters(cmux.HTTP2MatchHeaderFieldPrefixSendSettings("content-type", "application/grpc"))
		c.httpServer.listener = c.cmux.Match(cmux.Any())
		return nil
	}

	if c.httpServer != nil {
		c.httpServer.listener = listener
	}
	if c.grpc != nil {
		c.grpc.listener = listener
	}
	return nil
}

func (c *CoresService) listenCmux() mountFn {
	return mountFn{
		name: "BusinessMultiplexer",
		fn: func(context.Context) error {
			err := c.cmux.Serve()
			if c.stopping.Load() && isBusinessListenerClosed(err) {
				return nil
			}
			return err
		},
	}
}

func isBusinessListenerClosed(err error) bool {
	return errors.Is(err, net.ErrClosed) || errors.Is(err, cmux.ErrListenerClosed) || errors.Is(err, cmux.ErrServerClosed)
}

// businessListener 只保证共享监听器的 Close 幂等，不接管连接的生命周期。
type businessListener struct {
	net.Listener
	closeOnce sync.Once
	closeErr  error
}

// Close 会被 HTTP、gRPC 和 cmux 的退出路径重复调用，只关闭底层监听器一次。
// 缓存第一次关闭的结果，避免 HTTP Shutdown 完成排空后仍因重复关闭而误报 net.ErrClosed。
func (l *businessListener) Close() error {
	l.closeOnce.Do(func() { l.closeErr = l.Listener.Close() })
	return l.closeErr
}
