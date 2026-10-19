package mapclone

import "github.com/spf13/cast"

// CloneMap 深拷贝 map
func CloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}

	nm := make(map[string]any, len(m))
	for key, val := range m {
		switch v := val.(type) {
		case map[any]any:
			nm[key] = CloneMap(cast.ToStringMap(v))
		case map[string]any:
			nm[key] = CloneMap(v)
		default:
			nm[key] = val
		}
	}

	return nm
}
