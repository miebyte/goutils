package flags

import "github.com/spf13/pflag"

type IntGetter func() int

func (ig IntGetter) Value() int {
	if ig == nil {
		return 0
	}

	return ig()
}

func Int(key string, defaultVal int, usage string) IntGetter {
	pflag.Int(key, defaultVal, usage)
	sf.SetDefault(key, defaultVal)
	BindPFlag(key, pflag.Lookup(key))

	return func() int {
		return sf.GetInt(key)
	}
}

func IntRequired(key, usage string) IntGetter {
	requiredFlags = append(requiredFlags, key)
	return Int(key, 0, usage)
}
