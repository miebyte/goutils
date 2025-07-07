// File:		consul.go
// Created by:	Hoven
// Created on:	2025-06-05
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package consul

import (
	"fmt"

	"github.com/Masterminds/semver"
	"github.com/miebyte/goutils/flags/reader"
	"github.com/miebyte/goutils/internal/consul"
	"github.com/miebyte/goutils/internal/share"
	"github.com/miebyte/goutils/logging"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	_ "github.com/spf13/viper/remote"
)

type consulConfigReader struct {
}

func NewConsulConfigReader() *consulConfigReader {
	return &consulConfigReader{}
}

func listPossibleTags(tag string) []string {
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

func (cr *consulConfigReader) getServerKey(name string) string {
	return fmt.Sprintf("/etc/configs/%v", name)
}

func (cr *consulConfigReader) getRemotePossiblePath(name, tag string) string {
	var possiblePath string

	kv := consul.GetConsulClient().KV()
	for _, t := range listPossibleTags(tag) {
		path := fmt.Sprintf("%s/%s.yaml", cr.getServerKey(name), t)

		pair, _, err := kv.Get(path, nil)
		if err == nil && pair != nil {
			possiblePath = path
			break
		}
	}

	logging.Infof("Reading consul config from possiblePath(%s)", possiblePath)
	return possiblePath
}

func (cr *consulConfigReader) ReadConfig(v *viper.Viper, opt *reader.Option) error {
	if opt.ServiceName == "" || opt.Tag == "" {
		return errors.New("No service find. ServiceName and Tag is empty.")
	}

	logging.Infof("Reading consul config from Service(%s) Tag(%s)", opt.ServiceName, opt.Tag)
	path := cr.getRemotePossiblePath(opt.ServiceName, opt.Tag)
	if path == "" {
		return errors.New("No config found.")
	}

	v.AddRemoteProvider("consul", share.ConsulAddr(), path)
	v.SetConfigType("yaml")

	if err := v.ReadRemoteConfig(); err != nil {
		return errors.Wrap(err, "readRemoteConfig")
	}

	logging.Infof("Read remote config(%v:%v) success. Config=%v", opt.ServiceName, opt.Tag, path)
	return nil
}
