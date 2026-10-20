package flags

import "github.com/spf13/pflag"

type StringSliceGetter func() []string

func (sg StringSliceGetter) Value() []string {
	if sg == nil {
		return nil
	}

	return sg()
}

func StringSlice(key string, defaultVal []string, usage string) StringSliceGetter {
	pflag.StringSlice(key, defaultVal, usage)
	sf.SetDefault(key, defaultVal)
	BindPFlag(key, pflag.Lookup(key))

	return func() []string {
		return sf.GetStringSlice(key)
	}
}
