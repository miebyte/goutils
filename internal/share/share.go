// File:		share.go
// Created by:	Hoven
// Created on:	2025-04-08
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package share

import (
	"os"

	"github.com/miebyte/goutils/utils"
)

var (
	ServiceName func() string = func() string { return "" }
	Tag         func() string = func() string { return "" }
	Debug       func() bool   = func() bool { return false }
	UseConsul   func() bool   = func() bool { return false }
	ConsulAddr  func() string = func() string { return "" }
)

func SetServiceName(name string) {
	ServiceName = func() string { return name }
}

func SetTag(tag string) {
	Tag = func() string { return tag }
}

const (
	ProjectNameKey = "PROJECT_NAME"
	NamespaceKey   = "NAMESPACE"
	HostnameKey    = "HOSTNAME"
	PodIpKey       = "POD_IP"
)

var (
	HostName     = os.Getenv(HostnameKey)
	PodIp        = os.Getenv(PodIpKey)
	ProjectName  = utils.GetEnvByDefualt(ProjectNameKey, ProjectNameKey)
	PodNamespace = utils.GetEnvByDefualt(NamespaceKey, NamespaceKey)
)
