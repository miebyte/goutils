package flags

import (
	"strings"
	"sync"

	"github.com/miebyte/goutils/internal/innerlog"
)

var (
	keyStructMu  sync.RWMutex
	keyStructMap = make(map[string]ConfigReloadHook)
)

// ConfigReloadHook 表示配置热更新处理器。
type ConfigReloadHook interface {
	Reload()
}

// RegisterReloadFunc 为指定配置 key 注册热更新处理器，同一 key 只保留首次注册。
func RegisterReloadFunc(key string, r ConfigReloadHook) {
	key = strings.ToLower(key)

	keyStructMu.Lock()
	defer keyStructMu.Unlock()

	if _, exists := keyStructMap[key]; exists {
		innerlog.Logger.Infof("reload struct: %s has been registered", key)
		return
	}

	keyStructMap[key] = r
}

func doConfigHook(key string) {
	keyStructMu.RLock()
	rh, exists := keyStructMap[key]
	keyStructMu.RUnlock()
	if !exists {
		return
	}

	rh.Reload()
}

// TriggerReloadAll 触发全部已注册配置的热更新。
func TriggerReloadAll() {
	keyStructMu.RLock()
	hooks := make([]ConfigReloadHook, 0, len(keyStructMap))
	for _, h := range keyStructMap {
		hooks = append(hooks, h)
	}
	keyStructMu.RUnlock()

	for _, h := range hooks {
		h.Reload()
	}
}

// TriggerReload 触发指定 key 的热更新；key 为空时触发全部已注册处理器。
func TriggerReload(key string) {
	key = strings.ToLower(key)
	if key == "" {
		TriggerReloadAll()
		return
	}

	doConfigHook(key)
}
