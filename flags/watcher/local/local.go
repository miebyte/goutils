// File:		local.go
// Created by:	Hoven
// Created on:	2025-04-23
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package local

import (
	"github.com/fsnotify/fsnotify"
	"github.com/miebyte/goutils/flags/watcher"
	"github.com/miebyte/goutils/logging"
	"github.com/spf13/viper"
)

type LocalWatcher struct {
	callbacks []watchCallback
}

type watchCallback func()

func NewLocalWatcher() *LocalWatcher {
	return &LocalWatcher{
		callbacks: make([]watchCallback, 0),
	}
}

func (lw *LocalWatcher) SetCallbacks(callbacks ...watchCallback) {
	lw.callbacks = append(lw.callbacks, callbacks...)
}

func (lw *LocalWatcher) WatchConfig(v *viper.Viper, _ *watcher.Option) {
	v.WatchConfig()
	v.OnConfigChange(func(in fsnotify.Event) {
		logging.Debugf("local config change")
		for _, callback := range lw.callbacks {
			callback()
		}
	})
}
