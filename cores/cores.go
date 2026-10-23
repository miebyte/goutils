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
}

type CoresService struct {
	ctx    context.Context
	cancel func()

	serviceAlias string

	listenAddr       string
	listenPort       int
	listener         net.Listener
	httpMux          *http.ServeMux
	httpPattern      []httpPattern
	httpHandler      http.Handler
	httpCors         bool
	httpServerConfig *HttpServerConfig
	httpServer       *http.Server

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
		httpMux:          http.NewServeMux(),
		httpServerConfig: defaultHttpServerConfig,
		httpPattern:      make([]httpPattern, 0),
		mountFns:         make([]mountFn, 0),
		innerMountFns:    make([]mountFn, 0),
	}
	cs.httpHandler = cs.httpMux
	for _, opt := range opts {
		opt(cs)
	}

	return cs
}

func (c *CoresService) justWorker() bool {
	return c.listener == nil
}

func (c *CoresService) serve() error {
	if c.serviceAlias != "" {
		c.ctx = logging.With(c.ctx, "Alias", c.serviceAlias)
	}

	if share.ServiceName() != "" {
		c.ctx = logging.With(c.ctx, "Service", share.ServiceName())
	}

	c.innerMountFns = append(c.innerMountFns, c.gracefulKill())

	if c.justWorker() {
		return c.startServer()
	}

	c.setupPprof()

	c.innerMountFns = append(c.innerMountFns, c.listenHttp(c.listener))

	return c.startServer()
}

func (c *CoresService) startServer() error {
	c.wrapWorker()

	c.welcome()
	return c.runMountFn()
}

func (c *CoresService) runMountFn() error {
	runFn := func(mountFns []mountFn) *errgroup.Group {
		grp, ctx := errgroup.WithContext(c.ctx)
		for _, mountFn := range mountFns {
			grp.Go(func() (err error) {
				cctx := logging.With(ctx, "Worker", mountFn.name)
				err = waitContext(cctx, mountFn.maxWait, mountFn.waitAllDone, mountFn.fn)
				if err != nil {
					return errors.Wrap(err, "waitContext")
				}

				innerlog.Logger.Debugc(cctx, "worker run done")
				return nil
			})

			innerlog.Logger.Debugc(ctx, "worker(%s) start", mountFn.name)
		}

		return grp
	}

	workerGroup := runFn(c.mountFns)
	innerGroup := runFn(c.innerMountFns)

	workerErr := workerGroup.Wait()
	innerlog.Logger.Infoc(c.ctx, "all user worker run done")

	// 所有自定义 worker 运行完成
	// 触发 cores 结束
	if c.justWorker() {
		c.cancel()
	}

	// 等待所有内置 worker 运行完成
	innerErr := innerGroup.Wait()

	if workerErr == nil && innerErr == nil {
		return nil
	}

	return multierror.Append(workerErr, innerErr)
}

func Run(srv *CoresService) error {
	return srv.serve()
}

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
		return errors.Wrap(err, "failed to start listener")
	}

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	portInt, err := strconv.Atoi(port)
	if err == nil {
		srv.listenPort = portInt
	}

	srv.listener = listener
	srv.listenAddr = listener.Addr().String()
	return srv.serve()
}
