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
	"strings"

	"github.com/fullstorydev/grpcui/standalone"
	"github.com/miebyte/goutils/logging"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
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

func WithGRPCUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) ServiceOption {
	return func(c *CoresService) {
		c.grpcUnaryInterceptors = append(c.grpcUnaryInterceptors, interceptors...)
	}
}

func WithGRPCStreamInterceptors(interceptors ...grpc.StreamServerInterceptor) ServiceOption {
	return func(c *CoresService) {
		c.grpcStreamInterceptors = append(c.grpcStreamInterceptors, interceptors...)
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
	opts := []grpc.ServerOption{}

	unaryInterceptors := []grpc.UnaryServerInterceptor{}
	streamInterceptors := []grpc.StreamServerInterceptor{}

	unaryInterceptors = append(unaryInterceptors, c.grpcUnaryInterceptors...)

	streamInterceptors = append(streamInterceptors, c.grpcStreamInterceptors...)

	if len(unaryInterceptors) != 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(unaryInterceptors...))
	}

	if len(streamInterceptors) != 0 {
		opts = append(opts, grpc.ChainStreamInterceptor(streamInterceptors...))
	}

	opts = append(opts, c.grpcOptions...)

	c.grpcServer = grpc.NewServer(opts...)
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

func unaryServerLoggerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if strings.HasPrefix(info.FullMethod, "/grpc.reflection") {
		return handler(ctx, req)
	}

	prefix := strings.TrimPrefix(info.FullMethod, "/")
	ctx = logging.With(ctx, prefix)

	td := logging.TimeFuncDuration()
	ret, err := handler(ctx, req)
	duration := td()
	if err != nil {
		logging.Debugc(ctx, "Failed to handle method %s handle_time=%s handle_err=%s", info.FullMethod, duration, err)
	} else {
		logging.Debugc(ctx, "Succeed to handle method %s handle_time=%s", info.FullMethod, duration)
	}

	return ret, err
}

func streamServerLoggerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if strings.HasPrefix(info.FullMethod, "/grpc.reflection") {
		return handler(srv, ss)
	}

	ctx := ss.Context()

	prefix := strings.TrimPrefix(info.FullMethod, "/")
	ctx = logging.With(ctx, prefix)
	ss = newInjectServerStream(ctx, ss)

	td := logging.TimeFuncDuration()
	err := handler(srv, ss)
	duration := td()
	if err != nil {
		logging.Debugc(ctx, "Failed to handle method %s handle_time=%s handle_err=%s", info.FullMethod, duration, err)
	} else {
		logging.Debugc(ctx, "Succeed to handle method %s handle_time=%s", info.FullMethod, duration)
	}

	return err
}

type injectServerStream struct {
	ctx context.Context
	ss  grpc.ServerStream
}

func newInjectServerStream(ctx context.Context, ss grpc.ServerStream) *injectServerStream {
	return &injectServerStream{
		ss:  ss,
		ctx: ctx,
	}
}

func (ss *injectServerStream) SetHeader(md metadata.MD) error {
	return ss.ss.SetHeader(md)
}

func (ss *injectServerStream) SendHeader(md metadata.MD) error {
	return ss.ss.SendHeader(md)
}

func (ss *injectServerStream) SetTrailer(md metadata.MD) {
	ss.ss.SetTrailer(md)
}

func (ss *injectServerStream) Context() context.Context {
	return ss.ctx
}

func (ss *injectServerStream) SendMsg(m interface{}) error {
	return ss.ss.SendMsg(m)
}

func (ss *injectServerStream) RecvMsg(m interface{}) error {
	return ss.ss.RecvMsg(m)
}
