package logctx

import (
	"context"
	"fmt"
	"io"
)

type ContextKey string

var (
	LogContextKey ContextKey = "plogContext"
	LoggerKey     ContextKey = "plogLogger"

	badKey = "!BADKEY"
)

type LogContext struct {
	Group  []string
	Keys   []string
	Values []string
}

func GetLogContext(c context.Context) *LogContext {
	if c == nil {
		return nil
	}

	val := c.Value(LogContextKey)
	lc, ok := val.(*LogContext)
	if !ok {
		return nil
	}
	return lc
}

func ExtractLogger(ctx context.Context) io.Writer {
	if ctx == nil {
		return nil
	}
	i := ctx.Value(LoggerKey)
	if i == nil {
		return nil
	}
	w, ok := i.(io.Writer)
	if !ok {
		return nil
	}
	return w
}

func (logCtx *LogContext) ParseFmtKeyValue(msg string, v ...any) (c *LogContext, err error) {
	if len(v) == 0 {
		logCtx.Group = append(logCtx.Group, msg)
		return logCtx, nil
	}

	logCtx.Keys = append(logCtx.Keys, msg)
	logCtx.Values = append(logCtx.Values, fmt.Sprintf("%v", v[0]))

	if len(v) == 1 {
		return logCtx, nil
	}

	logCtx.Keys, logCtx.Values = parseRemains(logCtx.Keys, logCtx.Values, v[1:])

	return logCtx, nil
}

func parseRemains(keys, values []string, remains []any) ([]string, []string) {
	var key, val string
	for len(remains) > 0 {
		key, val, remains = getKVParis(remains)
		keys = append(keys, key)
		values = append(values, val)
	}

	return keys, values
}

func getKVParis(kvs []any) (string, string, []any) {
	switch x := kvs[0].(type) {
	case string:
		if len(kvs) == 1 {
			return badKey, x, nil
		}
		return x, fmt.Sprintf("%v", kvs[1]), kvs[2:]
	default:
		return badKey, fmt.Sprintf("%v", x), kvs[1:]
	}
}

func sliceClone(strSlice []string) []string {
	if strSlice == nil {
		return strSlice
	}
	return append(strSlice[:0:0], strSlice...)
}

func CloneLogContext(c *LogContext) *LogContext {
	if c == nil {
		return nil
	}
	clone := &LogContext{
		Group:  c.Group,
		Keys:   sliceClone(c.Keys),
		Values: sliceClone(c.Values),
	}

	return clone
}

func ReverseGroupWithPairs[T any](s []T) {
	n := len(s)

	groupCount := n / 2
	for i := 0; i < groupCount/2; i++ {
		left := i * 2
		right := (groupCount - 1 - i) * 2

		s[left], s[right] = s[right], s[left]
		s[left+1], s[right+1] = s[right+1], s[left+1]
	}
}
