// File:		share.go
// Created by:	Hoven
// Created on:	2025-04-08
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package share

var (
	ServiceName func() string = func() string { return "" }
	Debug       func() bool   = func() bool { return false }
	ConsulAddr  func() string = func() string { return "" }
)
