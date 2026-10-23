// File:		dialer.go
// Created by:	Hoven
// Created on:	2026-10-23
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package grpcutils

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func DialGrpcService(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	return dialGrpcWithTagContext(ctx, addr)
}

func dialGrpcWithTagContext(ctx context.Context, address string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	options := append(opts, defaultGRPCDialOptions()...)

	conn, err := grpc.NewClient(
		address,
		options...,
	)

	logger.Debugc(ctx, "dial grpc service Addr=%s", address)

	return conn, err

}

func defaultGRPCDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024 * 1024 * 16)),
		grpc.WithChainUnaryInterceptor(unaryClientLoggerInterceptor()),
		grpc.WithChainStreamInterceptor(streamClientLoggerInterceptor()),
	}

}
