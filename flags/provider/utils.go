package provider

import (
	"strings"

	"github.com/spf13/cast"
)

// normalizeMapCaseInsensitive normalizes the keys of a map to lower-case recursively.
func normalizeMapCaseInsensitive(m map[string]any) map[string]any {
	nm := make(map[string]any)
	for key, val := range m {
		lkey := strings.ToLower(key)
		switch v := val.(type) {
		case map[any]any:
			nm[lkey] = normalizeMapCaseInsensitive(cast.ToStringMap(v))
		case map[string]any:
			nm[lkey] = normalizeMapCaseInsensitive(v)
		default:
			nm[lkey] = v
		}
	}
	return nm
}
