// File:		main.go
// Created by:	Hoven
// Created on:	2025-04-22
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package main

import (
	"context"
	"os"

	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/logging/slog"
)

func main() {
	logging.SetLogger(slog.NewSlogJsonLogger(os.Stdout))
	ctx := logging.With(context.TODO(), "Group1")
	ctx = logging.With(ctx, "Group2")

	jsonMsg := map[string]any{
		"name": "hoven",
	}
	logging.Infoc(ctx, "this is info msg", "data", logging.JsonifyNoIndent(jsonMsg))
}
