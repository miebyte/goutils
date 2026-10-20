package flags

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

type doFunc[ARG any] func(ARG) error

type funcOption[ARG any] struct {
	m map[string]doFunc[ARG]
}

type optionFunc[ARG any] func(*funcOption[ARG])

func WithFunc[ARG any](key string, fn doFunc[ARG]) optionFunc[ARG] {
	return func(o *funcOption[ARG]) {
		o.m[key] = fn
	}
}

func Func[ARG any](key string, defaultVal string, usage string, opts ...optionFunc[ARG]) func(ARG) error {
	opt := &funcOption[ARG]{m: make(map[string]doFunc[ARG])}
	for _, o := range opts {
		o(opt)
	}

	supportKeys := make([]string, 0, len(opt.m))
	for k := range opt.m {
		supportKeys = append(supportKeys, k)
	}
	sort.Strings(supportKeys)
	usage = fmt.Sprintf("%s, support keys: %s.", usage, strings.Join(supportKeys, ", "))

	pflag.String(key, defaultVal, usage)
	sf.SetDefault(key, defaultVal)
	BindPFlag(key, pflag.Lookup(key))

	return func(arg ARG) error {
		val := sf.GetString(key)
		if val == "" {
			return errors.New("func key is empty")
		}

		if _, ok := opt.m[val]; !ok {
			return errors.New("func key not found")
		}

		return opt.m[val](arg)
	}
}
