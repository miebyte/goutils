// File:		grpc.go
// Created by:	Hoven
// Created on:	2025-06-09
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

	"github.com/fullstorydev/grpcui/standalone"
	"github.com/miebyte/goutils/logging"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

const (
	grpcuiUrl = "/debug/grpc/ui/"
)

func WithGrpcServer(grpcSrv func(srv *grpc.Server)) ServiceOption {
	return func(c *CoresService) {
		c.grpcServersFunc = append(c.grpcServersFunc, grpcSrv)
	}
}

func WithGrpcOptions(opt grpc.ServerOption) ServiceOption {
	return func(c *CoresService) {
		c.grpcOptions = append(c.grpcOptions, opt)
	}
}

func WithGrpcUI() ServiceOption {
	return func(c *CoresService) {
		c.grpcUIEnable = true
	}
}

func (c *CoresService) listenGrpc(lst net.Listener) mountFn {
	return mountFn{
		fn: func(ctx context.Context) error {
			return c.grpcServer.Serve(lst)
		},
		name: "GrpcListener",
	}
}

func (c *CoresService) grpcSelfConnect(lst net.Listener) error {
	_, port, _ := net.SplitHostPort(lst.Addr().String())
	target := fmt.Sprintf("127.0.0.1:%s", port)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(16 * 1024 * 1024)),
	}

	// conn, err := grpc.NewClient(target, opts...)
	conn, err := grpc.DialContext(c.ctx, target, opts...)
	if err != nil {
		return errors.Wrap(err, "self connect to grpc")
	}
	c.grpcSelfConn = conn
	return nil
}

func (c *CoresService) startGrpcServer() {
	c.grpcServer = grpc.NewServer(c.grpcOptions...)
	for _, fn := range c.grpcServersFunc {
		fn(c.grpcServer)
	}
	reflection.Register(c.grpcServer)

	if c.grpcUIEnable {
		logging.PanicError(c.grpcSelfConnect(c.listener))
	}
}

func (c *CoresService) mountGRPCUI() mountFn {
	return mountFn{
		fn: func(ctx context.Context) error {
			handler, err := standalone.HandlerViaReflection(c.ctx, c.grpcSelfConn, c.serviceName)
			if err != nil {
				return errors.Wrap(err, "start grpcUI")
			}

			c.httpMux.Handle(grpcuiUrl, http.StripPrefix("/debug/grpc/ui", handler))
			<-ctx.Done()
			return nil
		},
		name: "GRPCUI",
	}
}
