package flags

import (
	"time"

	"github.com/spf13/pflag"
)

func Duration(key string, defaultValue time.Duration, usage string) func() time.Duration {
	pflag.Duration(key, defaultValue, usage)
	sf.SetDefault(key, defaultValue)
	BindPFlag(key, pflag.Lookup(key))
	return func() time.Duration {
		return sf.GetDuration(key)
	}
}

func DurationRequired(key, usage string) func() time.Duration {
	requiredFlags = append(requiredFlags, key)
	return Duration(key, 0, usage)
}
