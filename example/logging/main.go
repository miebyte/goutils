// File:		main.go
// Created by:	Hoven
// Created on:	2025-08-20
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package main

import (
	"context"

	"github.com/miebyte/goutils/logging"
)

func main() {

	logging.Infof("this is info")
	logging.Infow(context.Background(), "this is info", logging.Int64("key", 1))
}
