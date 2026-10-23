// File:		share.go
// Created by:	Hoven
// Created on:	2025-04-08
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

// Package share 维护进程级共享信息，服务名由各模块统一读取。
package share

import (
	"strings"
	"sync/atomic"

	"github.com/miebyte/goutils/buildinfo"
	"github.com/miebyte/goutils/utils"
)

const (
	ServiceNameKey = "ServiceName"
	HostnameKey    = "HOSTNAME"
)

var (
	// 服务名由本包持有，不能再由配置模块替换为 Flag getter。
	// nil 表示尚未显式设置，此时沿用构建时注入的默认值。
	serviceName atomic.Pointer[string]
	Tag         func() string = func() string { return buildinfo.Version }
	Debug       func() bool   = func() bool { return false }

	HostName = utils.GetEnvByDefualt(HostnameKey, HostnameKey)
)

func init() {
	sn := utils.GetEnvByDefualt(ServiceNameKey, "")
	if sn == "" {
		return
	}

	segs := strings.SplitN(sn, ":", 2)
	if len(segs) >= 2 {
		SetServiceName(segs[0])
		SetTag(segs[1])
	} else {
		SetServiceName(sn)
	}
}

// SetServiceName 设置进程级服务名，优先于环境变量和构建注入值。
// 应在 flags.Parse、依赖初始化和 cores.Start 之前调用，运行期间不应更换服务名。
func SetServiceName(name string) {
	serviceName.Store(&name)
}

func SetTag(tag string) {
	Tag = func() string { return tag }
}

func GetHostName() string {
	return HostName
}

// GetServiceName 返回统一的进程服务名，配置路径、服务注册、日志和缓存均使用此值。
func GetServiceName() string {
	if name := serviceName.Load(); name != nil {
		return *name
	}
	return buildinfo.ServiceName
}
