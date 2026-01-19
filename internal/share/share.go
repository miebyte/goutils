// File:		share.go
// Created by:	Hoven
// Created on:	2025-04-08
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package share

import (
	"strings"

	"github.com/miebyte/goutils/utils"
)

const (
	ServiceNameKey = "ServiceName"
	HostnameKey    = "HOSTNAME"
)

var (
	ServiceName func() string
	Tag         func() string
	Debug       func() bool   = func() bool { return false }
	UseConsul   func() bool   = func() bool { return false }
	ConsulAddr  func() string = func() string { return "" }

	HostName = utils.GetEnvByDefualt(HostnameKey, HostnameKey)
)

func init() {
	sn := utils.GetEnvByDefualt(ServiceNameKey, ServiceNameKey)
	segs := strings.SplitN(sn, ":", 2)
	if len(segs) >= 2 {
		SetServiceName(segs[0])
		SetTag(segs[1])
	} else {
		SetServiceName(sn)
	}
}

func SetServiceName(name string) {
	ServiceName = func() string { return name }
}

func SetTag(tag string) {
	Tag = func() string { return tag }
}
