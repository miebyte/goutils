package superflags

import (
	"encoding/json"
	"slices"

	"github.com/miebyte/goutils/flags/v2/internal/mapclone"
	"github.com/spf13/cast"
)

func cloneValue(value any) any {
	switch v := value.(type) {
	case json.RawMessage:
		value = slices.Clone(v)
	case map[any]any:
		value = mapclone.CloneMap(cast.ToStringMap(v))
	case map[string]any:
		value = mapclone.CloneMap(v)
	}

	return value
}
