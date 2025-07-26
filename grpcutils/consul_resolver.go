// File:		consul_resolver.go
// Created by:	Hoven
// Created on:	2025-07-19
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package grpcutils

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/miebyte/goutils/internal/consul"
	"github.com/miebyte/goutils/logging"
	"google.golang.org/grpc/resolver"
)

var (
	_ resolver.Builder  = (*consulResolverBuilder)(nil)
	_ resolver.Resolver = (*consulResolver)(nil)
)

const (
	consulScheme = "goutils-consul"
)

type consulResolverBuilder struct{}

func (cr *consulResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &consulResolver{
		service: target.URL.Host,
		tag:     strings.TrimPrefix(target.URL.Path, "/"),
		cc:      cc,
	}
	r.start()
	return r, nil
}

func (cr *consulResolverBuilder) Scheme() string {
	return consulScheme
}

type consulResolver struct {
	service   string
	tag       string
	lastIndex uint64
	closed    bool
	lock      sync.Mutex
	cc        resolver.ClientConn
}

func (cr *consulResolver) srvTag() string {
	return strings.Join([]string{cr.service, cr.tag}, ":")
}

func (cr *consulResolver) start() {
	wg := sync.WaitGroup{}
	wg.Add(1)
	inited := false

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			cr.lock.Lock()
			if cr.closed {
				cr.lock.Unlock()
				logging.Debugf("Resolved loop terminated", cr.service, cr.tag)
				return
			}
			cr.lock.Unlock()

			cr.resolve()

			if !inited {
				wg.Done()
				inited = true
			}
		}
	}()

	wg.Wait()
}

func (cr *consulResolver) resolve() {
	var opt *api.QueryOptions
	if cr.lastIndex > 0 {
		opt = &api.QueryOptions{
			WaitIndex: cr.lastIndex,
		}
	}
	consulclient := consul.GetConsulClient()

	cs, meta, err := consulclient.Health().Service(cr.service, cr.tag, true, opt)
	if err != nil {
		logging.Errorf("Query consul", err)
		return
	}
	cr.lastIndex = meta.LastIndex

	addrs := make([]resolver.Address, len(cs))
	for i, s := range cs {
		// addr should like: 127.0.0.1:8001
		var addr string
		if s.Service.Address != "" {
			addr = fmt.Sprintf("%s:%d", s.Service.Address, s.Service.Port)
		} else {
			addr = fmt.Sprintf("%s:%d", s.Node.Address, s.Service.Port)
		}

		addrs[i] = resolver.Address{
			Addr:       addr,
			ServerName: cr.service,
			Metadata:   s.Service,
		}
	}
	logging.Debugf("Resolve service(%s) success", cr.srvTag())
	cr.cc.UpdateState(resolver.State{Addresses: addrs})
}

func (cr *consulResolver) ResolveNow(resolver.ResolveNowOptions) {}

func (cr *consulResolver) Close() {
	cr.lock.Lock()
	defer cr.lock.Unlock()
	cr.closed = true
	logging.Debugf("Resolved closed for service(%s) tag(%s)", cr.service, cr.tag)
}
