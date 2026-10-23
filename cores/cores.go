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
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/internal/share"
	"github.com/miebyte/goutils/logging"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"golang.org/x/sync/errgroup"

	_ "go.uber.org/automaxprocs"
)

type listenEntry interface {
	~int | ~string
}

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
	httpServers      []*httpServer
	httpCors         bool
	httpServerConfig *HttpServerConfig
	grpc             *grpcServer
	stopping         atomic.Bool

	workers       []Worker
	mountFns      []mountFn
	innerMountFns []mountFn
	waitAllDone   bool

	usePprof bool

	cron *cron.Cron
}

type ServiceOption func(*CoresService)

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
		internalHTTP:     &httpServer{mux: http.NewServeMux()},
		httpServerConfig: defaultHttpServerConfig,
		mountFns:         make([]mountFn, 0),
		innerMountFns:    make([]mountFn, 0),
	}
	for _, opt := range opts {
		opt(cs)
	}

	return cs
}

func (c *CoresService) justWorker() bool {
	return c.internalHTTP.listener == nil && len(c.httpServers) == 0 && c.grpc == nil
}

func (c *CoresService) serve() error {
	defer c.cancel()
	defer func() {
		if c.internalHTTP.listener != nil {
			_ = c.internalHTTP.listener.Close()
		}
		for _, srv := range c.httpServers {
			if srv.listener != nil {
				_ = srv.listener.Close()
			}
		}
		if c.grpc != nil && c.grpc.listener != nil {
			_ = c.grpc.listener.Close()
		}
	}()

	for _, srv := range c.httpServers {
		if srv.addr == "" {
			return errors.New("http listen address must not be empty")
		}
		for _, pattern := range srv.patterns {
			if pattern.handler == nil {
				return errors.New("http handler must not be nil")
			}
			srv.mux.Handle(pattern.pattern, http.StripPrefix(strings.TrimSuffix(pattern.pattern, "/"), pattern.handler))
		}
		listener, err := net.Listen("tcp", srv.addr)
		if err != nil {
			return errors.Wrapf(err, "failed to start http listener at %s", srv.addr)
		}
		srv.listener = listener
	}

	if c.grpc != nil {
		if c.grpc.server == nil {
			return errors.New("grpc server must not be nil")
		}
		if c.grpc.addr == "" {
			return errors.New("grpc listen address must not be empty")
		}
		listener, err := net.Listen("tcp", c.grpc.addr)
		if err != nil {
			return errors.Wrap(err, "failed to start grpc listener")
		}
		c.grpc.listener = listener
	}

	if c.serviceAlias != "" {
		c.ctx = logging.With(c.ctx, "Alias", c.serviceAlias)
	}

	if share.ServiceName() != "" {
		c.ctx = logging.With(c.ctx, "Service", share.ServiceName())
	}

	c.innerMountFns = append(c.innerMountFns, c.gracefulKill())

	if c.internalHTTP.listener != nil {
		c.setupPprof()
		c.internalHTTP.mux.Handle(healthCheckUrl, http.HandlerFunc(healthCheckApi))
		c.innerMountFns = append(c.innerMountFns, c.listenHttp(c.internalHTTP, "InternalHttpListener"))
	}
	for _, srv := range c.httpServers {
		c.innerMountFns = append(c.innerMountFns, c.listenHttp(srv, "HttpListener:"+srv.listener.Addr().String()))
	}

	if c.grpc != nil {
		c.innerMountFns = append(c.innerMountFns, c.listenGrpc())
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

	workerGroup := runFn(c.mountFns, true)
	innerGroup := runFn(c.innerMountFns, false)
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- workerGroup.Wait()
		if c.justWorker() {
			requestStop()
		}
	}()

	select {
	case <-stop:
	case <-c.ctx.Done():
	}

	shutdownErr := c.shutdownServers()
	c.cancel()
	workerErr := <-workerDone
	innerlog.Logger.Infoc(c.ctx, "all user worker run done")
	innerErr := innerGroup.Wait()

	return multierror.Append(workerErr, innerErr, shutdownErr).ErrorOrNil()
}

func Run(srv *CoresService) error {
	return srv.serve()
}

// Start runs the configured services and an internal HTTP server at addr.
// Business HTTP addresses are configured separately with WithHttpHandler.
func Start[T listenEntry](srv *CoresService, addr T) error {
	var address string
	switch v := any(addr).(type) {
	case int:
		address = ":" + strconv.Itoa(v)
	case string:
		address = v
	default:
		innerlog.Logger.Errorf("unsupport address type: %T", v)
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return errors.Wrap(err, "failed to start internal http listener")
	}

	srv.internalHTTP.listener = listener
	return srv.serve()
}
