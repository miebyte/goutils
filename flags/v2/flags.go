package flags

import (
	"time"

	"github.com/miebyte/goutils/flags/v2/internal/config"
	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/internal/share"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/logging/level"
	"github.com/miebyte/goutils/snail"
	"github.com/spf13/pflag"
)

var (
	sf            = New()
	requiredFlags = []string{}

	configFilePath StringGetter
)

type Option struct {
	UseRemote    bool
	RemoteOption RemoteConfigOption

	WatchConfig bool
	ConfigPath  string

	err error
}

type OptionFunc func(*Option)
type RemoteConfigOptionFunc func(*RemoteConfigOption)

func WithServiceName(name string) RemoteConfigOptionFunc {
	return func(o *RemoteConfigOption) {
		o.ServiceName = name
	}
}

type RemoteConfigOption struct {
	ServiceName string
	provider    ConfigProvider
}

// WithRemoteConfig 允许已声明的 Struct key 在 Parse 阶段统一从 Consul KV 读取 JSON。
// 普通 Flag 仍只使用命令行参数和默认值，不读取远程配置。
// 默认使用 SDK 默认配置及 CONSUL_* 环境变量，KV 路径为 /configs/{serviceName}/{key}。
// WithRemoteConsulProvider 可注入共用的 client。Getter 和 Struct Parser 只读取缓存。
// 显式 --configFile 优先使用本地文件，Struct 的远程选项在本地模式下不生效。
func WithRemoteConfig(opts ...RemoteConfigOptionFunc) OptionFunc {
	return func(o *Option) {
		o.UseRemote = true
		var rco RemoteConfigOption
		for _, opt := range opts {
			opt(&rco)
		}

		o.RemoteOption = rco
		if rco.ServiceName != "" {
			share.SetServiceName(rco.ServiceName)
			innerlog.Logger.Infof("set ServiceName as %s", rco.ServiceName)
		}
	}
}

// WithConfigWatch 开启所有 Struct 的配置监听；更新校验通过后直接发布新快照。
func WithConfigWatch() OptionFunc {
	return func(o *Option) {
		o.WatchConfig = true
	}
}

// WithSuperConfUp 为远程连接引导配置添加一个向上回退的候选目录；Consul 使用 SDK 配置，不受影响。
// 每份配置默认优先读取当前工作目录；仅文件不存在时，才尝试向上回退 n 层的目录，不逐层查找。
func WithSuperConfUp(n int) OptionFunc {
	return func(o *Option) {
		config.SetConfigUp(n)
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

	opt.ConfigPath = configFilePath()

	// 设置默认的 ConfigProvider
	// 显式文件优先，其次是远程来源，最后使用默认本地文件；自动发现不改变远程模式。
	source, err := getConfigProvider(opt)
	innerlog.Logger.PanicError(err)
	sf.SetConfigProvider(source)

	err = sf.ReadConfig()
	innerlog.Logger.PanicError(err)

	checkFlagKey()

	// Check the debug configuration again, this time from the config file.
	// If --debug is not used, it will rely on the debug setting in the config file.
	// If --debug is used, it will take precedence.
	setDebugMod()

	if opt.WatchConfig {
		innerlog.Logger.PanicError(sf.WatchConfig())
	}

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

func initOption(opts ...OptionFunc) *Option {
	opt := &Option{}

	for _, o := range opts {
		o(opt)
	}

	return opt
}

func initSuperFlags(opt *Option) {
	share.ServiceName = StringP("serviceName", "s", share.ServiceName(), "Set the service name.")
	share.Debug = Bool("debug", false, "Tag Whether to enable debug mode.")
	configFilePath = StringP("configFile", "f", opt.ConfigPath, "Specify config file. (JSON-only)")

	if err := sf.BindPFlags(pflag.CommandLine); err != nil {
		innerlog.Logger.Errorf("BindPflags error: %v", err)
	}
}

func checkFlagKey() {
	for _, rk := range requiredFlags {
		sf.mu.RLock()
		flag := sf.pflags[rk]
		sf.mu.RUnlock()

		value := sf.Get(rk)
		if flag != nil {
			switch flag.ValueType() {
			case "string":
				value = sf.GetString(rk)
			case "bool":
				value = sf.GetBool(rk)
			case "int":
				value = sf.GetInt(rk)
			case "float64":
				value = sf.GetFloat64(rk)
			case "duration":
				value = sf.GetDuration(rk)
			}
		}
		if isZero(value) {
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
	case int64:
		return it == 0
	case []string:
		return len(it) == 0
	case []any:
		return len(it) == 0
	default:
		return true
	}
}
