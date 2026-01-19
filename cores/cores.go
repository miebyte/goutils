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
	"strconv"
	"time"

	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/internal/share"
	"github.com/miebyte/goutils/logging"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type listenEntry interface {
	~int | ~string
}

type mountFn struct {
	name    string
	maxWait time.Duration
	fn      func(ctx context.Context) error
}

type CoresService struct {
	ctx    context.Context
	cancel func()

	serviceName  string
	tags         []string
	needRegister bool

	listenAddr string
	listener   net.Listener

	httpMux          *http.ServeMux
	httpServerConfig *HttpServerConfig
	httpPatterns     []string
	httpHandler      http.Handler
	httpCors         bool
	httpServer       *http.Server

	workers     []Worker
	mountFns    []mountFn
	waitAllDone bool

	usePprof      bool
	usePrometheus bool
}

type ServiceOption func(*CoresService)

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

func WithRegisterService() ServiceOption {
	return func(cs *CoresService) {
		cs.needRegister = true
	}
}

func NewCores(opts ...ServiceOption) *CoresService {
	ctx, cancel := context.WithCancel(context.TODO())

	cs := &CoresService{
		ctx:              ctx,
		cancel:           cancel,
		httpMux:          http.NewServeMux(),
		httpServerConfig: defaultHttpServerConfig,
		httpPatterns:     make([]string, 0),
		mountFns:         make([]mountFn, 0),
	}
	cs.httpHandler = cs.httpMux

	for _, opt := range opts {
		opt(cs)
	}

	return cs
}

func (c *CoresService) serve() error {
	c.injectServiceName()

	c.mountFns = []mountFn{c.gracefulKill()}
	if c.needRegister {
		c.mountFns = append(c.mountFns, c.registerService())
	}

	if c.listener == nil {
		return c.startServer()
	}

	c.mountFns = append(c.mountFns, c.listenHttp(c.listener))

	return c.startServer()
}

func (c *CoresService) injectServiceName() {
	if share.ServiceName() != "" {
		c.serviceName = share.ServiceName()
		c.ctx = logging.With(c.ctx, "Service", c.serviceName)
	}

	if share.Tag() != "" {
		c.tags = append(c.tags, share.Tag())
	}

	if len(c.tags) == 0 {
		c.tags = append(c.tags, "dev")
	}
}

func (c *CoresService) startServer() error {
	c.wrapWorker()
	c.setupPprof()
	c.setupPrometheus()

	c.welcome()
	return c.runMountFn()
}

func (c *CoresService) runMountFn() error {
	grp, ctx := errgroup.WithContext(c.ctx)

	for _, mount := range c.mountFns {
		mf := mount
		grp.Go(func() (err error) {
			cctx := logging.With(ctx, "Worker", mf.name)
			err = c.waitContext(cctx, mf.maxWait, mf.fn)
			if err != nil {
				return errors.Wrap(err, "waitContext")
			}
			return nil
		})
	}

	return grp.Wait()
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
	srv.listener = listener
	srv.listenAddr = listener.Addr().String()

	lstAddr, ok := listener.Addr().(*net.TCPAddr)
	if ok && lstAddr.IP.IsUnspecified() {
		srv.listenAddr = fmt.Sprintf("127.0.0.1:%v", lstAddr.Port)
	}

	return srv.serve()
}
