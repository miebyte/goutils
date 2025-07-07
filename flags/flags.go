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
	"strings"
	"time"

	"github.com/miebyte/goutils/discover"
	"github.com/miebyte/goutils/flags/reader"
	"github.com/miebyte/goutils/flags/watcher"
	"github.com/miebyte/goutils/internal/consul"
	"github.com/miebyte/goutils/internal/share"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/logging/level"
	"github.com/miebyte/goutils/snail"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	consulReader "github.com/miebyte/goutils/flags/reader/consul"
	localReader "github.com/miebyte/goutils/flags/reader/local"
	consulWatcher "github.com/miebyte/goutils/flags/watcher/consul"
	localWatcher "github.com/miebyte/goutils/flags/watcher/local"
)

const (
	projectNameKey = "PROJECT_NAME"
)

var (
	v                                          = viper.New()
	requiredFlags                              = []string{}
	nestedKey                                  = map[string]any{}
	defaultConfigName                          = "config"
	defaultConfigType                          = "yaml"
	defaultConfigReader  reader.ConfigReader   = localReader.NewLocalConfigReader()
	defaultConfigWatcher watcher.ConfigWatcher = localWatcher.NewLocalWatcher()

	config      StringGetter
	useRemote   BoolGetter
	watchConfig BoolGetter
)

func Viper() *viper.Viper {
	return v
}

func GetServiceName() string {
	return share.ServiceName()
}

type ParseOption func()

func WithConfigType(t string) ParseOption {
	return func() {
		defaultConfigType = t
	}
}

func WithConfigName(n string) ParseOption {
	return func() {
		defaultConfigName = n
	}
}

func Parse(opts ...ParseOption) {
	for _, opt := range opts {
		opt()
	}

	initViper()
	pflag.Parse()

	// reset project name while specify service name by flag
	os.Setenv(projectNameKey, share.ServiceName())
	parseService()

	if share.Debug() {
		logging.Enable(level.LevelDebug)
	}

	if useRemote() && config() == "" {
		checkServiceName()
		discover.SetFinder(consul.GetConsulClient())

		logging.Infof("Use Remote Config")
		defaultConfigReader = consulReader.NewConsulConfigReader()
		defaultConfigWatcher = consulWatcher.NewConsulWatcher()
	}

	if watchConfig() {
		startWatchConfig()
	}

	readConfig()
	checkFlagKey()

	snail.Init()
}

func parseService() {
	if share.ServiceName() == "" {
		return
	}

	segs := strings.SplitN(share.ServiceName(), ":", 2)
	if len(segs) >= 2 {
		share.SetServiceName(segs[0])
		share.SetTag(segs[1])
	} else {
		share.SetTag("dev")
	}
}

func checkServiceName() {
	if share.ServiceName() == "" {
		logging.Fatalf("ServiceName is empty, please use -s or --service to specify serviceName")
	}
}

func initViper() {
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")
	v.AddConfigPath(os.Getenv("HOME"))

	v.SetConfigName(defaultConfigName)
	v.SetConfigType(defaultConfigType)

	share.ServiceName = StringP("serviceName", "s", os.Getenv(projectNameKey), "Set the service name.")
	share.Debug = Bool("debug", false, "Whether to enable debug mode.")
	config = StringP("configFile", "f", "", "Specify config file. Support json, yaml, toml.")
	useRemote = Bool("remoteConfig", false, "True to use remote config.")
	watchConfig = Bool("watchConfig", false, "Set true to watch change on remote config.")

	if err := v.BindPFlags(pflag.CommandLine); err != nil {
		fmt.Println("BindPflags error", err)
	}
}

func startWatchConfig() {
	watchOpt := &watcher.Option{
		ServiceName: share.ServiceName(),
		Tag:         share.Tag(),
		ConfigPath:  config(),
	}

	defaultConfigWatcher.WatchConfig(v, watchOpt)
}

func readConfig() {
	readOpt := &reader.Option{
		ServiceName: share.ServiceName(),
		Tag:         share.Tag(),
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
