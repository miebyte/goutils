// File:		dialer.go
// Created by:	Hoven
// Created on:	2025-06-09
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package grpcutils

import (
	"context"
	"net"

	"github.com/miebyte/goutils/discover"
	"github.com/miebyte/goutils/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func DialGrpc(service string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	conn, err := dialGrpcWithTag(service, "", opts...)
	if err != nil {
		return nil, err
	}

	conn.Connect()
	return conn, nil
}

func DialGrpcWithUnBlock(service string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return dialGrpcWithTag(service, "", opts...)
}

func DialGrpcWithTag(service string, tag string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return dialGrpcWithTag(service, tag, opts...)
}

func dialGrpcWithTag(service, tag string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	options := append(opts, defaultGRPCDialOptions(tag)...)
	options = append(options)

	conn, err := grpc.NewClient(
		service,
		options...,
	)

	conn.Connect()

	return conn, err
}

func defaultGRPCDialOptions(tag string) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithContextDialer(grpcConnDialFn(tag)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024 * 1024 * 16)),
		grpc.WithChainUnaryInterceptor(unaryClientLoggerInterceptor()),
		grpc.WithChainStreamInterceptor(streamClientLoggerInterceptor()),
	}
}

func grpcConnDialFn(tag string) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, service string) (net.Conn, error) {
		address := discover.GetServiceFinder().GetAddressWithTag(service, tag)

		if tag != "" {
			logging.Debugc(ctx, "dial grpc service %s with tag %s. Addr=%s", service, tag, address)
		} else {
			logging.Debugc(ctx, "dial grpc service %s. Addr=%s", service, address)
		}

		return net.Dial("tcp", address)
	}
}
