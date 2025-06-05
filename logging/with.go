// File:		with.go
// Created by:	Hoven
// Created on:	2025-04-03
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package logging

import (
	"context"
	"io"

	"github.com/miebyte/goutils/logging/logctx"
	"github.com/miebyte/goutils/masking"
)

// With used to store some data in log-ctx
// Ie supports two forms of writing
// 1. With(log-ctx, "group")
// When only msg has a value, it is used as a group
// 2. With(log-ctx, "key1", "value1") or With(log-ctx, "key1", "value1", "key2", "value2")
// When msg and v both have values, With resolves them into key-value pairs
// The parsed results are stored in the LogContext for use by the Logger
func With(c context.Context, msg string, v ...any) context.Context {
	if c == nil {
		c = context.Background()
	}

	if msg == "" && len(v) == 0 {
		return c
	}

	lc := logctx.GetLogContext(c)
	if lc == nil {
		lc = &logctx.LogContext{}
	}

	newLc := logctx.CloneLogContext(lc)

	newLc, err := newLc.ParseFmtKeyValue(msg, v...)
	if err != nil {
		Errorf("with parse error: %v", err)
		return c
	}

	for idx, value := range newLc.Values {
		newLc.Values[idx] = masking.MaskMessage(value)
	}

	return context.WithValue(c, logctx.LogContextKey, newLc)
}

func WithLogger(c context.Context, w io.Writer) context.Context {
	return context.WithValue(c, logctx.LoggerKey, w)
}
