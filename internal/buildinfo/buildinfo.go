// File:		buildinfo.go
// Created by:	Hoven
// Created on:	2025-04-04
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package buildinfo

import (
	"os"
	"runtime/debug"
)

var (
	buildInfo  *debug.BuildInfo
	modulePath string
	host       string
)

func init() {
	var ok bool
	buildInfo, ok = debug.ReadBuildInfo()
	if !ok {
		return
	}

	modulePath = buildInfo.Main.Path
	host, _ = os.Hostname()
}

func GetModuleVersion() string {
	return buildInfo.Main.Version
}

func GetModulePath() string {
	return modulePath
}

func GetHost() string {
	return host
}
