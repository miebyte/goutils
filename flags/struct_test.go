package flags

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStruct(t *testing.T) {
	structConf := Struct("structConf", (*map[string]any)(nil), "struct config")

	sf.Set("structConf", `{"name":"hoven"}`)

	resp := make(map[string]any)

	err := structConf(&resp)
	assert.Nil(t, err)
	assert.Equal(t, map[string]any{"name": "hoven"}, resp)
}

type reloadTestConf struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (c *reloadTestConf) SetDefault() {
	if c.Count == 0 {
		c.Count = 1
	}
}

func (c *reloadTestConf) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name required")
	}
	return nil
}

func (c *reloadTestConf) Reload() {
	reloadTestHookCount++
}

var reloadTestHookCount int

func TestStructReloadKeepsLastValidConfig(t *testing.T) {
	const key = "reload_struct_conf"
	reloadTestHookCount = 0
	t.Cleanup(func() {
		reloadTestHookCount = 0
		keyStructMu.Lock()
		delete(keyStructMap, key)
		keyStructMu.Unlock()
	})

	parser := Struct(key, (*reloadTestConf)(nil), "reload struct config")
	sf.ReplaceKey(key, map[string]any{"name": "init", "count": 2})

	cfg := new(reloadTestConf)
	require.NoError(t, parser(cfg))
	require.Equal(t, "init", cfg.Name)
	require.Equal(t, 2, cfg.Count)
	require.Equal(t, 0, reloadTestHookCount)

	sf.ReplaceKey(key, map[string]any{"name": "ok", "count": 5})
	TriggerReload(key)
	assert.Equal(t, "ok", cfg.Name)
	assert.Equal(t, 5, cfg.Count)
	assert.Equal(t, 1, reloadTestHookCount)

	sf.ReplaceKey(key, map[string]any{"name": "", "count": 9})
	TriggerReload(key)
	assert.Equal(t, "ok", cfg.Name)
	assert.Equal(t, 5, cfg.Count)
	assert.Equal(t, 1, reloadTestHookCount)

	sf.ReplaceKey(key, map[string]any{"name": "defaulted"})
	TriggerReload(key)
	assert.Equal(t, "defaulted", cfg.Name)
	assert.Equal(t, 1, cfg.Count)
	assert.Equal(t, 2, reloadTestHookCount)
}
