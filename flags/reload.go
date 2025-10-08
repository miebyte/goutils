package flags

import "github.com/miebyte/goutils/internal/innerlog"

var (
	keyStructMap = make(map[string]ConfigReloadHook)
)

type ConfigReloadHook interface {
	Reload()
}

func RegisterReloadFunc(key string, r ConfigReloadHook) {
	if _, exists := keyStructMap[key]; exists {
		innerlog.Logger.Infof("reload struct: %s has been registered", key)
		return
	}

	keyStructMap[key] = r
}

func doConfigHook(key string) {
	rh, exists := keyStructMap[key]
	if !exists {
		return
	}

	rh.Reload()
}

func TriggerReloadAll() {
	for key := range keyStructMap {
		doConfigHook(key)
	}
}

// TriggerReload invokes reload hook if registered for the given key.
func TriggerReload(key string) {
	if key == "" {
		TriggerReloadAll()
		return
	}

	doConfigHook(key)
}
