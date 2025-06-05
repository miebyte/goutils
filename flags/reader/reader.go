// File:		reader.go
// Created by:	Hoven
// Created on:	2025-04-01
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package reader

import (
	"github.com/spf13/viper"
)

type Option struct {
	ServiceName string
	Tag         string
	ConfigPath  string
}

type ConfigReader interface {
	ReadConfig(v *viper.Viper, opt *Option) error
}
