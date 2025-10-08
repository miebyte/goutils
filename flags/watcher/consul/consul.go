// File:		consul.go
// Created by:	Hoven
// Created on:	2025-06-06
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package consulWatcher

import (
	"bytes"
	"fmt"

	"github.com/Masterminds/semver"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
	"github.com/miebyte/goutils/consulutils"
	"github.com/miebyte/goutils/flags/watcher"
	"github.com/miebyte/goutils/internal/share"
	"github.com/miebyte/goutils/logging"
	"github.com/spf13/viper"
)

type ConsulWatcher struct {
	callbacks []watchCallback
}

type watchCallback func()

func NewConsulWatcher() *ConsulWatcher {
	return &ConsulWatcher{
		callbacks: make([]watchCallback, 0),
	}
}

func (cw *ConsulWatcher) SetCallbacks(callbacks ...watchCallback) {
	cw.callbacks = append(cw.callbacks, callbacks...)
}

func (cw *ConsulWatcher) getServerKey(name string) string {
	return fmt.Sprintf("/etc/configs/%v", name)
}

func (cw *ConsulWatcher) listPossibleTags(tag string) []string {
	v, err := semver.NewVersion(tag)
	if err != nil {
		return []string{tag}
	}

	majorV := v.Major()
	minorV := v.Minor()
	patchV := v.Patch()

	possibleTags := []string{tag}
	for i := int(patchV - 1); i >= 0; i-- {
		possibleTags = append(possibleTags, fmt.Sprintf("v%d.%d.%d", majorV, minorV, i))
	}

	possibleTags = append(possibleTags, fmt.Sprintf("v%d.%d", majorV, minorV))

	return possibleTags
}

func (cw *ConsulWatcher) getRemotePossiblePath(name, tag string) string {
	var possiblePath string

	kv := consulutils.GetConsulClient().KV()
	for _, t := range cw.listPossibleTags(tag) {
		path := fmt.Sprintf("%s/%s.yaml", cw.getServerKey(name), t)

		pair, _, err := kv.Get(path, nil)
		if err == nil && pair != nil {
			possiblePath = path
			break
		}
	}

	return possiblePath
}

func (cw *ConsulWatcher) WatchConfig(v *viper.Viper, opt *watcher.Option) {
	key := cw.getRemotePossiblePath(opt.ServiceName, opt.Tag)
	p, err := watch.Parse(map[string]interface{}{"type": "key", "key": key})
	logging.PanicError(err)

	first := true
	var currentVal []byte

	p.Handler = func(index uint64, data interface{}) {
		kv, ok := (data).(*api.KVPair)
		if !ok {
			logging.Errorf("Failed to watch remote config data.")
			return
		}
		// There is always a trigger at first launch, ignore it.
		if first {
			first = false
			currentVal = kv.Value
			return
		}
		if string(kv.Value) == string(currentVal) {
			logging.Warnf("Remote index changed, but value was not changed")
			return
		}
		logging.Debugf("Remote config changed")

		if err := v.MergeConfig(bytes.NewReader(kv.Value)); err != nil {
			logging.Warnf("Remote changed, but failed to merge. data_size=%d err=%s", len(kv.Value), err)
			return
		}

		currentVal = kv.Value
		for _, callback := range cw.callbacks {
			callback()
		}
	}

	go func() {
		logging.PanicError(p.Run(share.ConsulAddr()))
	}()

	logging.Debugf("Start watch consul config(%s)", key)
}
