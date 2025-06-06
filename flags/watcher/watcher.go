// File:		watcher.go
// Created by:	Hoven
// Created on:	2025-04-23
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package watcher

import "github.com/spf13/viper"

type Option struct {
	ServiceName string
	Tag         string
	ConfigPath  string
}

type ConfigWatcher interface {
	WatchConfig(v *viper.Viper, opt *Option)
}
