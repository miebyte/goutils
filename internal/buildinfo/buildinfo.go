// File:		buildinfo.go
// Created by:	Hoven
// Created on:	2025-04-04
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package buildinfo

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/hashicorp/go-version"
)

const (
	goutilsPath = "github.com/miebyte/goutils"
)

var (
	buildInfo     *debug.BuildInfo
	modulePath    string
	moduleVersion string
	host          string
	args          []string
)

func initModuleInfo() {
	var ok bool
	buildInfo, ok = debug.ReadBuildInfo()
	if !ok {
		return
	}

	modulePath = buildInfo.Main.Path
	for _, dep := range buildInfo.Deps {
		if dep.Path != goutilsPath {
			continue
		}

		moduleVersion = dep.Version
		break
	}

	if moduleVersion != "" {
		if v, err := version.NewSemver(moduleVersion); err == nil {
			if prerelease := v.Prerelease(); prerelease != "" {
				fmt.Printf("[❌警告❌] 当前使用的是预发布版本: %s (预发布标识: %s), 请尽快切换到正式版本\n", moduleVersion, prerelease)
			}
		}
	}
}

func init() {
	args = os.Args
	host, _ = os.Hostname()
	initModuleInfo()
}

func GetModuleVersion() string {
	return moduleVersion
}

func GetModulePath() string {
	return modulePath
}

func GetHost() string {
	return host
}

func GetArgs() []string {
	return args
}
