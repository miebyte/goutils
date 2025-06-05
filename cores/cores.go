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
	"time"

	"github.com/miebyte/goutils/internal/share"
	"github.com/miebyte/goutils/logging"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type listenerGetter func() net.Listener

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

	serviceName string
	tags        []string

	listenAddr  string
	listener    net.Listener
	httpMux     *http.ServeMux
	httpPattern string
	httpHandler http.Handler
	httpCors    bool

	workers     []Worker
	mountFns    []mountFn
	waitAllDone bool

	sentryMonitor bool
	useKafkaLog   bool
	usePprof      bool

	useTracing bool
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

func NewCores(opts ...ServiceOption) *CoresService {
	ctx, cancel := context.WithCancel(context.TODO())

	cs := &CoresService{
		ctx:      ctx,
		cancel:   cancel,
		httpMux:  http.NewServeMux(),
		mountFns: make([]mountFn, 0),
	}
	cs.httpHandler = cs.httpMux

	for _, opt := range opts {
		opt(cs)
	}

	return cs
}

func (c *CoresService) serve() error {
	if share.ServiceName() != "" {
		segs := strings.SplitN(share.ServiceName(), ":", 2)
		if len(segs) < 2 {
			c.serviceName = share.ServiceName()
		} else {
			c.serviceName = segs[0]
			c.tags = append(c.tags, segs[1])
		}
		c.ctx = logging.With(c.ctx, "Service", share.ServiceName())
	}

	if len(c.tags) == 0 {
		c.tags = append(c.tags, "dev")
	}

	c.mountFns = []mountFn{
		c.gracefulKill(),
		c.registerService(),
	}

	if c.listener != nil {
		c.mountFns = append(c.mountFns, c.listenHttp())
	}

	c.wrapWorker()
	c.setupPprof()

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
		logging.Errorf("unsupport address type: %T", v)
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return errors.Wrap(err, "failed to start listener")
	}

	srv.listener = listener
	srv.listenAddr = listener.Addr().String()
	return srv.serve()
}
