// File:		flags.go
// Created by:	Hoven
// Created on:	2025-04-01
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package flags

import (
	"os"
	"time"

	"github.com/miebyte/goutils/discover"
	"github.com/miebyte/goutils/flags/provider"
	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/internal/share"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/logging/level"
	"github.com/miebyte/goutils/snail"
	"github.com/spf13/pflag"
)

var (
	sf                                   = New()
	requiredFlags                        = []string{}
	defaultConfigName                    = "config"
	defaultConfigType                    = "json"
	defaultConfigProvider ConfigProvider = nil

	config StringGetter
)

type Option struct {
	UseRemote   BoolGetter
	WatchConfig BoolGetter
}

type OptionFunc func(opt *Option)

func WithConfigWatch() OptionFunc {
	return func(opt *Option) {
		opt.WatchConfig = func() bool { return true }
	}
}

func WithUseRemote() OptionFunc {
	return func(opt *Option) {
		opt.UseRemote = func() bool { return true }
	}
}

func GetServiceName() string {
	return share.ServiceName()
}

// Parse is used to parse the command line arguments and the configuration file.
// it just can be called once or it will panic.
func Parse(opts ...OptionFunc) {
	opt := initOption(opts...)

	initSuperFlags(opt)
	pflag.Parse()

	setDebugMod()

	if opt.UseRemote() {
		if config() != "" {
			defaultConfigProvider = provider.NewLocalProvider(config())
		} else {
			checkServiceName()
			defaultConfigProvider = provider.NewConsulProvider(share.ServiceName())
			discover.SetConsulFinder()
		}
	} else {
		configPath := config()
		if configPath == "" {
			configPath = sf.FindConfigFile()
			innerlog.Logger.Debugf("find local config file: %s", configPath)
		}

		defaultConfigProvider = provider.NewLocalProvider(configPath)
	}

	sf.SetConfigProvider(defaultConfigProvider)

	readConfig(opt)
	watchConfig(opt)
	checkFlagKey()

	// Check the debug configuration again, this time from the config file.
	// If --debug is not used, it will rely on the debug setting in the config file.
	// If --debug is used, it will take precedence.
	setDebugMod()

	snail.Init()
}

func setDebugMod() {
	var lev level.Level
	if share.Debug() {
		lev = level.LevelDebug
	} else {
		lev = level.LevelInfo
	}
	innerlog.Logger.Enable(lev)
	logging.Enable(lev)
}

func checkServiceName() {
	if share.ServiceName() == "" {
		innerlog.Logger.Fatalf("ServiceName is empty, please use -s or --service to specify serviceName")
	}
}

func initOption(opts ...OptionFunc) *Option {
	opt := &Option{}

	for _, o := range opts {
		o(opt)
	}

	return opt
}

func initSuperFlags(opt *Option) {
	sf.AddConfigPath(".")
	sf.AddConfigPath("./configs")
	sf.AddConfigPath(os.Getenv("HOME"))

	sf.SetConfigName(defaultConfigName)
	sf.SetConfigType(defaultConfigType)
	share.ServiceName = StringP("serviceName", "s", share.ServiceName(), "Set the service name.")
	share.Tag = StringP("serviceTag", "t", share.Tag(), "Set the service tag.")
	share.Debug = Bool("debug", false, "Tag whether to enable debug mode.")
	opt.UseRemote = Bool("useRemote", false, "Tag whether to use remote config")
	opt.WatchConfig = Bool("watchConfig", false, "Tag whether to watch config")
	config = StringP("configFile", "f", "", "Specify config file. (JSON-only)")

	if err := sf.BindPFlags(pflag.CommandLine); err != nil {
		innerlog.Logger.Errorf("BindPflags error: %v", err)
	}
}

func readConfig(_ *Option) {
	err := sf.ReadConfig()
	innerlog.Logger.PanicError(err)
}

func watchConfig(opt *Option) {
	if !opt.WatchConfig() {
		return
	}

	ch := sf.WatchConfig()
	if ch == nil {
		return
	}

	go func() {
		for ev := range ch {
			innerlog.Logger.Debugf("watch config change: %s, config: %v", ev.Key, ev.Config)
			if ev.Key == "" {
				sf.ReplaceConfig(ev.Config)
			} else {
				sf.ReplaceKey(ev.Key, ev.Config)
			}
			TriggerReload(ev.Key)
		}
	}()
}

func checkFlagKey() {
	for _, rk := range requiredFlags {
		if isZero(sf.Get(rk)) {
			innerlog.Logger.Fatalf("Missing key: %s", rk)
		}
	}
}

func isZero(i any) bool {
	switch it := i.(type) {
	case bool:
		// It's trivial to check a bool, since it makes the flag no sense(always true).
		return !it
	case string:
		return it == ""
	case time.Duration:
		return it == 0
	case float64:
		return it == 0
	case int:
		return it == 0
	case []string:
		return len(it) == 0
	case []any:
		return len(it) == 0
	default:
		return true
	}
}
