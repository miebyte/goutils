package flags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStruct(t *testing.T) {
	structConf := Struct("structConf", (*map[string]any)(nil), "struct config")

	sf.Set("structConf", `{"name":"hoven"}`)

	resp := make(map[string]any)

	err := structConf(&resp)
	assert.Nil(t, err)
	assert.Equal(t, map[string]any{"name": "hoven"}, resp)
}
