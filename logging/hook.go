package logging

import (
	"fmt"
	"maps"
	"sync"

	"github.com/miebyte/goutils/logging/level"
)

var (
	globalHooks   LevelHook = make(LevelHook)
	globalHooksMu sync.RWMutex
)

type Hook interface {
	Name() string
	Levels() []level.Level
	Fire(*Entry) error
}

type LevelHook map[level.Level]map[string]Hook

func (lh LevelHook) IsEmpty() bool {
	return len(lh) == 0
}

func (lh LevelHook) Fire(level level.Level, entry *Entry) error {
	for _, hook := range lh[level] {
		if err := hook.Fire(entry); err != nil {
			return fmt.Errorf("fire hook(%s) error: %w", hook.Name(), err)
		}
	}

	return nil
}

func AddGlobalHook(hook Hook) {
	globalHooksMu.Lock()
	defer globalHooksMu.Unlock()

	for _, lev := range hook.Levels() {
		m, exists := globalHooks[lev]
		if !exists {
			m = make(map[string]Hook)
			globalHooks[lev] = m
		}

		m[hook.Name()] = hook
	}
}

func FireGlobalHook(level level.Level, entry *Entry) error {
	globalHooksMu.RLock()
	tmpHooks := make(LevelHook, len(globalHooks))
	maps.Copy(tmpHooks, globalHooks)
	globalHooksMu.RUnlock()

	return tmpHooks.Fire(level, entry)
}
