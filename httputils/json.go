package httputils

import (
	"encoding/json"

	"github.com/miebyte/goutils/logging"
)

type JsonBody map[string]any

func NewJsonBody() JsonBody {
	return make(JsonBody)
}

func (j JsonBody) Add(key string, value any) JsonBody {
	_, exists := j[key]
	if !exists {
		j[key] = value
	}
	return j
}

func (j JsonBody) Encode() []byte {
	b, err := json.Marshal(j)
	if err != nil {
		logging.Errorf("json body encode error: %v", err)
		return nil
	}

	return b
}
