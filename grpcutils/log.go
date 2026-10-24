// File:		log.go
// Created by:	Hoven
// Created on:	2026-10-23
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package grpcutils

import (
	"context"
	"os"

	"github.com/miebyte/goutils/logging"
	"google.golang.org/grpc"
)

var (
	logger *logging.PrettyLogger
)

func init() {
	logger = logging.NewPrettyLogger(os.Stdout, logging.WithModule("GrpcUtils"))
	logger.SetWithSource(false)
}

func unaryClientLoggerInterceptor() func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if !logging.IsDebug() {
			return invoker(ctx, method, req, reply, cc, opts...)

		}
		td := logging.TimeFuncDuration()
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			logger.Infoc(ctx, "Failed to invoke method %s invoke_time=%s invoke_err=%s", method, td(), err)

		} else {
			logger.Infoc(ctx, "Succeed to invoke method %s invoke_time=%s", method, td())

		}
		return err

	}

}

func streamClientLoggerInterceptor() func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if !logging.IsDebug() {
			return streamer(ctx, desc, cc, method, opts...)

		}

		td := logging.TimeFuncDuration()
		cs, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			logger.Infoc(ctx, "Failed to invoke method %s invoke_time=%s invoke_err=%s", method, td(), err)

		} else {
			logger.Infoc(ctx, "Succeed to invoke method %s invoke_time=%s", method, td())

		}
		return cs, err

	}
}
