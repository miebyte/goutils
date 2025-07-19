// File:		dialer.go
// Created by:	Hoven
// Created on:	2025-06-09
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package grpcutils

import (
	"fmt"

	"github.com/miebyte/goutils/internal/share"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func DialGrpc(service string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	conn, err := dialGrpcWithTag(service, "", opts...)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func DialGrpcWithUnBlock(service string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return dialGrpcWithTag(service, "", opts...)
}

func DialGrpcWithTag(service string, tag string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return dialGrpcWithTag(service, tag, opts...)
}

func dialGrpcWithTag(service, tag string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	options := append(opts, defaultGRPCDialOptions()...)

	target := fmt.Sprintf("%s://%s/%s", consulScheme, service, tag)
	conn, err := grpc.NewClient(
		target,
		options...,
	)

	return conn, err
}

func defaultGRPCDialOptions() []grpc.DialOption {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024 * 1024 * 16)),
		grpc.WithChainUnaryInterceptor(unaryClientLoggerInterceptor()),
		grpc.WithChainStreamInterceptor(streamClientLoggerInterceptor()),
	}

	if share.UseConsul() {
		opts = append(opts, grpc.WithResolvers(&consulResolverBuilder{}))
	}

	return opts
}
