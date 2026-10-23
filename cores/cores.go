// File:		cores.go
// Created by:	Hoven
// Created on:	2025-04-02
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package cores

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/miebyte/goutils/consulutils"
	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/share"
	"github.com/miebyte/goutils/utils"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"github.com/soheilhy/cmux"
	"golang.org/x/sync/errgroup"

	_ "go.uber.org/automaxprocs"
)

type mountFn struct {
	name        string
	maxWait     time.Duration
	waitAllDone bool
	fn          func(ctx context.Context) error
}

type httpPattern struct {
	pattern string
	name    string
	handler http.Handler
}

type CoresService struct {
	ctx    context.Context
	cancel func()

	serviceAlias string

	internalHTTP     *httpServer
	httpServer       *httpServer
	serviceAddress   string
	configErr        error
	businessListener net.Listener
	cmux             cmux.CMux
	httpCors         bool
	httpServerConfig *HttpServerConfig
	grpc             *grpcServer
	stopping         atomic.Bool
	ready            atomic.Bool
	serviceMode      bool
	consulConfig     *consulutils.RegistrationConfig

	workers       []Worker
	mountFns      []mountFn
	innerMountFns []mountFn
	waitAllDone   bool

	usePprof bool

	cron *cron.Cron
}

type ServiceOption func(*CoresService)

// WithServiceAddress 统一配置唯一业务监听地址，HTTP 与 gRPC 共同使用。
// 管理地址仍由 WithInternalHTTPAddress 单独配置；不要为不同协议设置多个业务地址。
func WithServiceAddress(addr string) ServiceOption {
	return func(cs *CoresService) {
		if cs.serviceAddress != "" && cs.serviceAddress != addr {
			cs.configErr = errors.New("only one business listen address is supported")
			return
		}
		cs.serviceAddress = addr
	}
}

func WithServiceAlias(name string) ServiceOption {
	return func(cs *CoresService) {
		cs.serviceAlias = name
	}
}

func WithDefaultMaxWait(wait time.Duration) ServiceOption {
	return func(_ *CoresService) {
		defaultMaxWait = wait
	}
}

func WithWaitAllDone() ServiceOption {
	return func(cs *CoresService) {
		cs.waitAllDone = true
	}
}

func WithHttpServerConfig(config *HttpServerConfig) ServiceOption {
	return func(cs *CoresService) {
		cs.httpServerConfig = config
	}
}

func NewCores(opts ...ServiceOption) *CoresService {
	ctx, cancel := context.WithCancel(context.TODO())

	cs := &CoresService{
		ctx:              ctx,
		cancel:           cancel,
		internalHTTP:     &httpServer{addr: defaultInternalHTTPAddress, mux: http.NewServeMux()},
		httpServerConfig: defaultHttpServerConfig,
		mountFns:         make([]mountFn, 0),
		innerMountFns:    make([]mountFn, 0),
	}
	for _, opt := range opts {
		opt(cs)
	}

	return cs
}

func (c *CoresService) serve() error {
	defer c.cancel()
	defer func() {
		if c.internalHTTP.listener != nil {
			_ = c.internalHTTP.listener.Close()
		}
		if c.businessListener != nil {
			_ = c.businessListener.Close()
		}
	}()

	if c.configErr != nil {
		return c.configErr
	}

	// 服务模式统一提供管理端口；脚本模式不会创建任何监听器。
	// 放在同一清理范围内，后续业务端口绑定失败时也会释放管理端口。
	if c.serviceMode {
		// 启动内置的 http服务
		// 用于绑定健康检查接口，pprof 接口等
		if c.internalHTTP.addr == "" {
			return errors.New("internal http listen address must not be empty")
		}

		listener, err := net.Listen("tcp", c.internalHTTP.addr)
		if err != nil {
			return errors.Wrap(err, "failed to start internal http listener")
		}

		c.internalHTTP.listener = listener
	}

	// 业务挂载了 http 服务
	if c.httpServer != nil {
		patterns := make(map[string]struct{}, len(c.httpServer.patterns))

		for _, pattern := range c.httpServer.patterns {
			if pattern.handler == nil {
				return errors.New("http handler must not be nil")
			}

			// 统一前缀后再检查重复，避免多组挂载导致 ServeMux 直接 panic。
			if _, exists := patterns[pattern.pattern]; exists {
				return errors.Errorf("duplicate HTTP API prefix: %s", pattern.pattern)
			}

			patterns[pattern.pattern] = struct{}{}
			c.httpServer.mux.Handle(pattern.pattern, http.StripPrefix(strings.TrimSuffix(pattern.pattern, "/"), pattern.handler))
		}

		innerlog.Logger.Infoc(c.ctx, "HttpHandler enabled. %v", utils.MapKeys(patterns))
	}

	// 业务挂载了 grpc 服务
	if c.grpc != nil {
		innerlog.Logger.Infoc(c.ctx, "GrpcHandler enabled.")
	}

	// HTTP 与 gRPC 由同一个业务监听器承载，协议分流与管理 HTTP 完全独立。
	err := c.setupBusinessListener()
	if err != nil {
		return fmt.Errorf("failed to setup business listener: %w", err)
	}

	if c.serviceAlias != "" {
		c.ctx = logging.With(c.ctx, "Alias", c.serviceAlias)
	}

	if share.GetServiceName() != "" {
		c.ctx = logging.With(c.ctx, "Service", share.GetServiceName())
	}

	c.innerMountFns = append(c.innerMountFns, c.gracefulKill())

	// 挂载内部 http 服务
	if c.internalHTTP.listener != nil {
		c.setupPprof()
		c.internalHTTP.mux.Handle(healthCheckUrl, http.HandlerFunc(healthCheckApi))
		c.internalHTTP.mux.Handle(readyCheckUrl, http.HandlerFunc(c.readyCheckApi))
		c.innerMountFns = append(c.innerMountFns, c.listenHttp(c.internalHTTP, "InternalHttpListener"))
	}

	// 挂载业务 http 服务
	if c.httpServer != nil {
		c.innerMountFns = append(c.innerMountFns, c.listenHttp(c.httpServer, "HttpListener:"+c.httpServer.listener.Addr().String()))
	}

	// 挂载 grpc 服务
	if c.grpc != nil {
		c.innerMountFns = append(c.innerMountFns, c.listenGrpc())
	}

	// 如果使用了端口复用，则需要挂载 cmux 服务
	if c.cmux != nil {
		c.innerMountFns = append(c.innerMountFns, c.listenCmux())
	}

	return c.startServer()
}

func (c *CoresService) startServer() error {
	c.wrapWorker()
	c.welcome()
	return c.runMountFn()
}

func (c *CoresService) runMountFn() error {
	stop := make(chan struct{}, 1)
	requestStop := func() {
		select {
		case stop <- struct{}{}:
		default:
		}
	}

	runFn := func(mountFns []mountFn, userWorker bool) *errgroup.Group {
		grp := new(errgroup.Group)
		for _, mountFn := range mountFns {
			grp.Go(func() (err error) {
				cctx := logging.With(c.ctx, "Worker", mountFn.name)
				if userWorker {
					err = waitContext(cctx, mountFn.maxWait, mountFn.waitAllDone, mountFn.fn)
				} else {
					err = mountFn.fn(cctx)
				}
				if err != nil && !errors.Is(err, context.Canceled) {
					requestStop()
					return errors.Wrapf(err, "%s run failed", mountFn.name)
				}
				if !userWorker {
					requestStop()
				}

				innerlog.Logger.Debugc(cctx, "worker run done")
				return nil
			})

			innerlog.Logger.Debugc(c.ctx, "worker(%s) start", mountFn.name)
		}

		return grp
	}

	innerGroup := runFn(c.innerMountFns, false)
	// 所有监听器均已绑定，监听任务启动后才对外宣布就绪。
	// 业务依赖在调用 Start 前初始化，Worker 不承担启动就绪屏障。
	c.ready.Store(c.serviceMode)
	stopRegistration := c.startConsulRegistration()
	workerGroup := runFn(c.mountFns, true)
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- workerGroup.Wait()
		if !c.serviceMode {
			requestStop()
		}
	}()

	select {
	case <-stop:
	case <-c.ctx.Done():
	}

	// 在关闭监听器之前先摘除就绪状态、停止注册重试并注销自身实例。
	c.ready.Store(false)
	c.stopping.Store(true)
	if stopRegistration != nil {
		stopRegistration()
	}
	shutdownErr := c.shutdownServers()
	c.cancel()
	workerErr := <-workerDone
	innerlog.Logger.Infoc(c.ctx, "all user worker run done")
	innerErr := innerGroup.Wait()

	return multierror.Append(workerErr, innerErr, shutdownErr).ErrorOrNil()
}

// Run 执行脚本模式：只运行 Worker，全部任务完成后退出。
// 脚本不启动管理 HTTP 或 Consul 注册；需要长期提供服务时使用 Start。
func Run(srv *CoresService) error {
	if srv.httpServer != nil || srv.grpc != nil {
		return errors.New("script mode does not support HTTP or gRPC listeners; use cores.Start")
	}
	srv.serviceMode = false
	return srv.serve()
}

// Start 执行服务模式，始终启动默认 :6060 的管理 HTTP 服务。
// 管理地址通过 WithInternalHTTPAddress 覆盖，业务监听地址由各自的选项配置。
func Start(srv *CoresService) error {
	srv.serviceMode = true
	return srv.serve()
}
