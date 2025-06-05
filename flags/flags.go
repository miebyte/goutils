// File:		flags.go
// Created by:	Hoven
// Created on:	2025-04-01
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package flags

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/miebyte/goutils/flags/reader"
	"github.com/miebyte/goutils/internal/share"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/logging/level"
	"github.com/miebyte/goutils/snail"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	localReader "github.com/miebyte/goutils/flags/reader/local"
)

const (
	projectNameKey = "PROJECT_NAME"
	namespaceKey   = "NAMESPACE"
)

var (
	v                                       = viper.New()
	requiredFlags                           = []string{}
	nestedKey                               = map[string]any{}
	defaultConfigName                       = "config"
	defaultConfigType                       = "json"
	defaultConfigReader reader.ConfigReader = localReader.NewLocalConfigReader()

	config StringGetter
)

type Option struct {
	UseRemote bool
}

type OptionFunc func(*Option)

func Viper() *viper.Viper {
	return v
}

func GetServiceName() string {
	return share.ServiceName()
}

func Parse(opts ...OptionFunc) {
	opt := initOption(opts...)

	initViper(opt)
	pflag.Parse()

	// reset project name while specify service name by flag
	os.Setenv(projectNameKey, share.ServiceName())

	if share.Debug() {
		logging.Enable(level.LevelDebug)
	}

	if opt.UseRemote && config() == "" {
		checkServiceName()
		// TODO: set consul reader

	}

	readConfig(opt)
	checkFlagKey()

	snail.Init()
}

func checkServiceName() {
	if share.ServiceName() == "" {
		logging.Fatalf("ServiceName is empty, please use -s or --service to specify serviceName")
	}
}

func initOption(opts ...OptionFunc) *Option {
	opt := &Option{}

	for _, o := range opts {
		o(opt)
	}

	return opt
}

func initViper(_ *Option) {
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")
	v.AddConfigPath(os.Getenv("HOME"))

	v.SetConfigName(defaultConfigName)
	v.SetConfigType(defaultConfigType)

	share.ServiceName = StringP("serviceName", "s", os.Getenv(projectNameKey), "Set the service name")
	share.Debug = Bool("debug", false, "Whether to enable debug mode.")
	config = StringP("configFile", "f", "", "Specify config file. Support json, yaml, toml.")

	if err := v.BindPFlags(pflag.CommandLine); err != nil {
		fmt.Println("BindPflags error", err)
	}
}

func readConfig(_ *Option) {
	readOpt := &reader.Option{
		ServiceName: share.ServiceName(),
		ConfigPath:  config(),
	}

	err := defaultConfigReader.ReadConfig(v, readOpt)
	if err != nil {
		panic(err)
	}
}

func checkFlagKey() {
	for _, rk := range requiredFlags {
		if isZero(v.Get(rk)) {
			log.Fatalf("Missing key: %s", rk)
		}
	}
}

func isZero(i interface{}) bool {
	switch i.(type) {
	case bool:
		// It's trivial to check a bool, since it makes the flag no sense(always true).
		return !i.(bool)
	case string:
		return i.(string) == ""
	case time.Duration:
		return i.(time.Duration) == 0
	case float64:
		return i.(float64) == 0
	case int:
		return i.(int) == 0
	case []string:
		return len(i.([]string)) == 0
	case []interface{}:
		return len(i.([]interface{})) == 0
	default:
		return true
	}
}
