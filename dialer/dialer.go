// File:		dialer.go
// Created by:	Hoven
// Created on:	2025-04-25
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package dialer

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/snail"
	"github.com/miebyte/goutils/flags"
)

type ServiceConfigMap map[string]any

var (
	serviceConf ServiceConfigMap
	isInit      bool
)

func init() {
	snail.RegisterObject("readServiceConfig", func() error {
		serviceFlag := flags.Struct("service", (ServiceConfigMap)(nil), "service config map")

		serviceConf := make(ServiceConfigMap)
		if err := serviceFlag(serviceConf); err != nil {
			logging.Debugf("read service config failed: %v", err)
			return nil
		}

		isInit = true
		return nil
	})
}

func DialService[T any](servicename string) (T, error) {
	var result T

	if !isInit {
		return result, errors.New("service conf not init")
	}

	conf, exists := serviceConf[servicename]
	if !exists {
		return result, errors.Wrap(ErrServiceNotExists, servicename)
	}

	data, err := json.Marshal(conf)
	if err != nil {
		return result, errors.Wrap(err, "marshal service config failed")
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return result, errors.Wrap(err, "unmarshal service config failed")
	}

	return result, nil
}
