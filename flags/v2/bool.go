package superflags

import "github.com/spf13/pflag"

type BoolGetter func() bool

func (bg BoolGetter) Value() bool {
	if bg == nil {
		return false
	}

	return bg()
}

func Bool(key string, defaultVal bool, usage string) BoolGetter {
	pflag.Bool(key, defaultVal, usage)
	sf.SetDefault(key, defaultVal)
	BindPFlag(key, pflag.Lookup(key))

	return func() bool {
		return sf.GetBool(key)
	}
}

func BoolP(key, shorthand string, defaultVal bool, usage string) BoolGetter {
	pflag.BoolP(key, shorthand, defaultVal, usage)
	sf.SetDefault(key, defaultVal)
	BindPFlag(key, pflag.Lookup(key))

	return func() bool {
		return sf.GetBool(key)
	}
}

func BoolRequired(key, usage string) BoolGetter {
	requiredFlags = append(requiredFlags, key)
	return Bool(key, false, usage)
}
