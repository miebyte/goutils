package flags

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/miebyte/goutils/flags/provider"
	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/spf13/afero"
	"github.com/spf13/cast"
	"github.com/spf13/pflag"
)

type ConfigProvider interface {
	ReadConfig() (map[string]any, error)
	WatchConfig() <-chan provider.Event
}

// SuperFlags is a lightweight configuration and flag manager.
// It searches in pflags, then in loaded config, then in defaults.
type SuperFlags struct {
	// A set of paths to look for the config file in
	configPaths    []string
	findConfigOnce sync.Once
	findConfigPath string

	// The filesystem to read config from.
	fs afero.Fs

	// Name of file to look for inside the path
	configName string
	configFile string
	configType string

	config   map[string]any
	defaults map[string]any
	pflags   map[string]FlagValue

	// integrated components
	configProvider ConfigProvider

	mu sync.RWMutex
}

func New() *SuperFlags {
	sf := new(SuperFlags)
	sf.configName = "config"
	sf.fs = afero.NewOsFs()
	// init maps
	sf.config = make(map[string]any)
	sf.defaults = make(map[string]any)
	sf.pflags = make(map[string]FlagValue)

	return sf
}

func (sf *SuperFlags) SetConfigFile(in string) {
	if in != "" {
		sf.mu.Lock()
		sf.configFile = in
		sf.mu.Unlock()
	}
}

func (sf *SuperFlags) SetConfigName(in string) {
	if in != "" {
		sf.mu.Lock()
		sf.configName = in
		sf.configFile = ""
		sf.mu.Unlock()
	}
}

func (sf *SuperFlags) SetConfigType(in string) {
	if in != "" {
		sf.mu.Lock()
		sf.configType = in
		sf.mu.Unlock()
	}
}

func (sf *SuperFlags) SetDefault(key string, value any) {
	key = strings.ToLower(key)
	value = toCaseInsensitiveValue(value)

	sf.mu.Lock()
	sf.defaults[key] = value
	sf.mu.Unlock()
}

func (sf *SuperFlags) AddConfigPath(in string) {
	if in != "" {
		absin := absPathify(in)

		innerlog.Logger.Debugf("adding path to search paths, path: %s", absin)
		sf.mu.Lock()
		if !slices.Contains(sf.configPaths, absin) {
			sf.configPaths = append(sf.configPaths, absin)
		}
		sf.mu.Unlock()
	}
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
	sf.pflags[strings.ToLower(key)] = flag
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

func (sf *SuperFlags) ReplaceConfig(newCfg map[string]any) {
	if newCfg == nil {
		return
	}

	// normalize
	norm := copyAndInsensitiviseMap(newCfg)
	sf.mu.Lock()
	sf.config = norm
	sf.mu.Unlock()
}

func (sf *SuperFlags) ReplaceKey(key string, value any) {
	if value == nil {
		return
	}

	lkey := strings.ToLower(key)
	val := toCaseInsensitiveValue(value)

	sf.mu.Lock()
	sf.config[lkey] = val
	sf.mu.Unlock()
}

func (sf *SuperFlags) Get(key string) any {
	lcaseKey := strings.ToLower(key)
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	// search sf.Pfalgs first
	if flag, ok := sf.pflags[lcaseKey]; ok {
		if flag.HasChanged() {
			return flag.ValueString()
		}
	}

	// search sf.Config next
	if val, ok := sf.config[lcaseKey]; ok {
		return val
	}

	// search sf.Defaults next
	if val, ok := sf.defaults[lcaseKey]; ok {
		return val
	}

	return nil
}

func (sf *SuperFlags) Set(key string, value string) {
	lkey := strings.ToLower(key)
	sf.mu.Lock()
	if p, exists := sf.pflags[lkey]; exists {
		p.Set(value)
	}

	sf.config[lkey] = value
	sf.mu.Unlock()
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

// FindConfigFile searches known config paths for a config file named
// {configName}.{configType}. If SetConfigFile was called and the file exists,
// it returns that path first. Returns empty string if none found.
func (sf *SuperFlags) FindConfigFile() string {
	sf.findConfigOnce.Do(func() {
		// snapshot fields under read lock to avoid races
		sf.mu.RLock()
		cf := sf.configFile
		cn := sf.configName
		ct := sf.configType
		paths := append([]string(nil), sf.configPaths...)
		fsys := sf.fs
		sf.mu.RUnlock()

		// explicit config file path wins if it exists
		if cf != "" {
			if _, err := fsys.Stat(cf); err == nil {
				sf.mu.Lock()
				sf.findConfigPath = cf
				// also reflect into internal config map for downstream watchers
				sf.config["configfile"] = cf
				sf.mu.Unlock()
				return
			}
		}

		if cn == "" || ct == "" {
			return
		}
		filename := fmt.Sprintf("%s.%s", cn, ct)

		for _, dir := range paths {
			candidate := filepath.Join(dir, filename)
			if _, err := fsys.Stat(candidate); err == nil {
				sf.mu.Lock()
				sf.findConfigPath = candidate
				// set discovered path into config map (lower-cased key)
				sf.config["configfile"] = candidate
				sf.mu.Unlock()
				return
			}
		}
	})
	return sf.findConfigPath
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

func (sf *SuperFlags) SetConfigProvider(r ConfigProvider) {
	sf.mu.Lock()
	sf.configProvider = r
	sf.mu.Unlock()
}

// ReadConfig reads configuration via the integrated reader and replaces in-memory config.
func (sf *SuperFlags) ReadConfig() error {
	sf.mu.RLock()
	r := sf.configProvider
	sf.mu.RUnlock()
	if r == nil {
		return nil
	}
	cfg, err := r.ReadConfig()
	if err != nil {
		return err
	}
	if cfg != nil {
		sf.ReplaceConfig(cfg)
	}
	return nil
}

// WatchConfig starts watching configuration changes using the integrated watcher.
func (sf *SuperFlags) WatchConfig() <-chan provider.Event {
	sf.mu.RLock()
	w := sf.configProvider
	sf.mu.RUnlock()
	if w == nil {
		return nil
	}
	return w.WatchConfig()
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
	if err := sf.BindPFlag(key, flag); err != nil {
		innerlog.Logger.Errorf("BindPFlag key: %v, err: %v", key, err)
	}
}
