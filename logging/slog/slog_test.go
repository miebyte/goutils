// File:		slog_test.go
// Created by:	Hoven
// Created on:	2025-04-24
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package slog_test

import (
	"context"
	"os"
	"testing"

	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/logging/slog"
)

func TestSlog(t *testing.T) {
	logger := slog.NewSlogPrettyLogger(os.Stdout)
	ctx := context.Background()
	ctx = logging.With(ctx, "Group")
	logger.Infof("test", "name", "hoven")
	ctx = logging.With(ctx, "Group2")
	logger.Infof("test", "name", "hoven")

	logger.Infoc(ctx, "this is ctx info", "age", 18)

	m := map[string]any{
		"name": "hoven",
		"age":  18,
	}
	logger.Infof("this is map info: %v", logging.JsonifyNoIndent(m))

	ctx = logging.With(ctx, "city", "shenzhen")
	logger.Infoc(ctx, "this is ctx info with city.")

	ctx = logging.With(ctx, "country", "china")
	logger.Infoc(ctx, "this is ctx info with city and country.")

	ctx = logging.With(ctx, "province", "guangdong")
	logger.Infoc(ctx, "this is ctx info with city, country and province.")

	for i := 0; i < 10; i++ {
		logger.Infoc(ctx, "this is ctx info in for loop.")
	}
}
