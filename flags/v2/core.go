package flags

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miebyte/goutils/flags/v2/internal/local"
	"github.com/miebyte/goutils/flags/v2/provider"
	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/spf13/cast"
	"github.com/spf13/pflag"
)

// SuperFlags 是一个配置和标志管理器。
// 用于从指定的 ConfigProvider 读取配置并存放起来
// 并将命令行解析的标志绑定到 SuperFlags 中，因此可以获取到命令行标志指定的配置
type SuperFlags struct {
	// structKeyReader  用于存储使用 Struct 读取配置的 key。
	// 由于使用 Struct 可以指定不同的 Provider，所以需要一个 map 来存储。
	structKeyReaders map[string]keyReader
	// normalKeys 用于存储使用普通 Flag 读取配置的 key。
	normalKeys []string

	config                map[string]json.RawMessage
	defaults              map[string]any
	pflags                map[string]FlagValue
	defaultConfigProvider provider.ConfigProvider

	isLocal   bool
	watchOnce sync.Once
	watchErr  error
	watchers  sync.WaitGroup
	closeOnce sync.Once
	closeErr  error

	mu sync.RWMutex
}

func New() *SuperFlags {
	sf := new(SuperFlags)
	// init maps
	sf.config = make(map[string]json.RawMessage)
	sf.defaults = make(map[string]any)
	sf.pflags = make(map[string]FlagValue)
	sf.structKeyReaders = make(map[string]keyReader)
	sf.normalKeys = make([]string, 0)

	return sf
}

func (sf *SuperFlags) SetDefault(key string, value any) {
	value = cloneValue(value)

	sf.mu.Lock()
	sf.defaults[key] = value
	sf.mu.Unlock()
}

func (sf *SuperFlags) BindPFlags(flags *pflag.FlagSet) error {
	return sf.BindFlagValues(pflagValueSet{flags})
}

func (sf *SuperFlags) BindPFlag(key string, flag *pflag.Flag) error {
	if flag == nil {
		return fmt.Errorf("flag for %q is nil", key)
	}
	return sf.BindFlagValue(key, pflagValue{flag})
}

func (sf *SuperFlags) BindFlagValue(key string, flag FlagValue) error {
	if flag == nil {
		return fmt.Errorf("flag for %q is nil", key)
	}
	sf.mu.Lock()
	if _, exists := sf.pflags[key]; !exists {
		sf.normalKeys = append(sf.normalKeys, key)
	}
	sf.pflags[key] = flag
	sf.mu.Unlock()
	return nil
}

func (sf *SuperFlags) BindFlagValues(flags FlagValueSet) (err error) {
	var firstErr error
	flags.VisitAll(func(flag FlagValue) {
		if firstErr != nil {
			return
		}
		if e := sf.BindFlagValue(flag.Name(), flag); e != nil {
			firstErr = e
		}
	})
	return firstErr
}

func (sf *SuperFlags) BindNormalKey(key string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	sf.normalKeys = append(sf.normalKeys, key)
}

func (sf *SuperFlags) BindStructKey(opt structOption, confRefresh func(json.RawMessage) error) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if _, exists := sf.structKeyReaders[opt.key]; exists {
		innerlog.Logger.PanicError(fmt.Errorf("Struct %q has already been declared", opt.key))
	}

	sf.structKeyReaders[opt.key] = keyReader{
		key:         opt.key,
		path:        opt.path,
		isFullPath:  opt.isFullPath,
		provider:    opt.provider,
		isStruct:    true,
		confRefresh: confRefresh,
	}
}

func (sf *SuperFlags) ReplaceConfig(newCfg map[string]json.RawMessage) {
	if newCfg == nil {
		return
	}

	config := make(map[string]json.RawMessage, len(newCfg))
	for key, value := range newCfg {
		config[key] = slices.Clone(value)
	}
	sf.mu.Lock()
	sf.config = config
	sf.mu.Unlock()
}

func (sf *SuperFlags) Get(key string) any {
	value := sf.get(key)
	raw, ok := value.(json.RawMessage)
	if !ok {
		return value
	}

	var decoded any
	err := json.Unmarshal(raw, &decoded)
	if err != nil {
		return string(raw)
	}

	_, ok = decoded.(float64)
	if ok {
		// 整数从原文解析，避免经过 float64 后丢失精度。
		integer, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
		if err == nil {
			return integer
		}
	}
	return decoded
}

// get 用于根据 key 从已读取的配置中获取值
// 优先从 pflags 中获取，然后从 config 中获取，最后从 defaults 中获取
func (sf *SuperFlags) get(key string) any {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	// search sf.Pfalgs first
	if flag, ok := sf.pflags[key]; ok {
		if flag.HasChanged() {
			return flag.ValueString()
		}
	}

	// search sf.Config next
	if val, ok := sf.config[key]; ok {
		return val
	}

	// search sf.Defaults next
	if val, ok := sf.defaults[key]; ok {
		return val
	}

	return nil
}

func (sf *SuperFlags) GetString(key string) string {
	return cast.ToString(sf.Get(key))
}

func (sf *SuperFlags) GetBool(key string) bool {
	return cast.ToBool(sf.Get(key))
}

func (sf *SuperFlags) GetInt(key string) int {
	return cast.ToInt(sf.Get(key))
}

func (sf *SuperFlags) GetFloat64(key string) float64 {
	return cast.ToFloat64(sf.Get(key))
}

func (sf *SuperFlags) GetDuration(key string) time.Duration {
	return cast.ToDuration(sf.Get(key))
}

func (sf *SuperFlags) GetStringSlice(key string) []string {
	sf.mu.RLock()
	flag, ok := sf.pflags[key].(pflagValue)
	sf.mu.RUnlock()
	if ok && flag.HasChanged() && flag.ValueType() == "stringSlice" {
		if value, ok := flag.flag.Value.(pflag.SliceValue); ok {
			// pflag 的展示值是 CSV，不能按 JSON 重新解释转义字符。
			return slices.Clone(value.GetSlice())
		}
	}

	val := sf.Get(key)
	switch v := val.(type) {
	case []string:
		return v
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
			var arr []string
			if json.Unmarshal([]byte(s), &arr) == nil {
				return arr
			}
			inner := strings.TrimSpace(s[1 : len(s)-1])
			r := csv.NewReader(strings.NewReader(inner))
			if fields, err := r.Read(); err == nil {
				return fields
			}
		}
		r := csv.NewReader(strings.NewReader(s))
		if fields, err := r.Read(); err == nil {
			return fields
		}
		return cast.ToStringSlice(s)
	default:
		return cast.ToStringSlice(val)
	}
}

// FlagValueSet is an interface that users can implement
// to bind a set of flags to viper.
type FlagValueSet interface {
	VisitAll(fn func(FlagValue))
}

// FlagValue is an interface that users can implement
// to bind different flags to viper.
type FlagValue interface {
	Set(value string)
	HasChanged() bool
	Name() string
	ValueString() string
	ValueType() string
}

func (sf *SuperFlags) SetConfigProvider(r provider.ConfigProvider) {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	_, isLocal := r.(*local.LocalProvider)

	sf.isLocal = isLocal
	sf.defaultConfigProvider = r
}

// ReadConfig 统一读取已登记的 key，全部成功后再替换缓存；读取失败时保留原缓存。
// 只有明确缺失的配置才跳过，让 Getter 使用默认值；普通 Flag 允许纯文本，Struct 校验 JSON。
// normalKeys 的配置，仅从本地文件获取
func (sf *SuperFlags) ReadConfig() error {
	sf.mu.RLock()
	r := sf.defaultConfigProvider

	readers := make(map[string]keyReader, len(sf.normalKeys)+len(sf.structKeyReaders))
	// 仅当存在本地 provider 的时候才读取 normalKeys 的配置
	// 如果没有本地 provider，则这些 normalKeys 将仅从 pflags 中获取
	if sf.isLocal {
		for _, key := range sf.normalKeys {
			readers[key] = keyReader{key: key, provider: r}
		}
	}
	maps.Copy(readers, sf.structKeyReaders)

	sf.mu.RUnlock()
	if r == nil {
		return nil
	}

	newConf := make(map[string]json.RawMessage, len(readers))
	for key, reader := range readers {
		raw, err := reader.ReadConfig(context.Background(), r)
		if errors.Is(err, provider.ErrConfigNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read config %q: %w", key, err)
		}

		if len(raw) == 0 {
			continue
		}

		if reader.isStruct && !json.Valid(raw) {
			return fmt.Errorf("read config %q: invalid JSON", key)
		}

		newConf[key] = slices.Clone(raw)
	}

	sf.ReplaceConfig(newConf)
	return nil
}

// pflagValueSet is a wrapper around *pflag.ValueSet
// that implements FlagValueSet.
type pflagValueSet struct {
	flags *pflag.FlagSet
}

// VisitAll iterates over all *pflag.Flag inside the *pflag.FlagSet.
func (p pflagValueSet) VisitAll(fn func(flag FlagValue)) {
	p.flags.VisitAll(func(flag *pflag.Flag) {
		fn(pflagValue{flag})
	})
}

// pflagValue is a wrapper around *pflag.flag
// that implements FlagValue.
type pflagValue struct {
	flag *pflag.Flag
}

func (p pflagValue) Set(value string) {
	p.flag.Changed = true
	p.flag.Value.Set(value)
}

// HasChanged returns whether the flag has changes or not.
func (p pflagValue) HasChanged() bool {
	return p.flag.Changed
}

// Name returns the name of the flag.
func (p pflagValue) Name() string {
	return p.flag.Name
}

// ValueString returns the value of the flag as a string.
func (p pflagValue) ValueString() string {
	return p.flag.Value.String()
}

// ValueType returns the type of the flag as a string.
func (p pflagValue) ValueType() string {
	return p.flag.Value.Type()
}

func BindPFlag(key string, flag *pflag.Flag) {
	err := sf.BindPFlag(key, flag)
	if err != nil {
		innerlog.Logger.Errorf("BindPFlag key: %v, err: %v", key, err)
	}
}
