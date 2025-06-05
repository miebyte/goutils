// File:		watcher.go
// Created by:	Hoven
// Created on:	2025-04-23
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package watcher

import "github.com/spf13/viper"

type ConfigWatcher interface {
	WatchConfig(v *viper.Viper, path string)
}
