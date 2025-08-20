package logging

import "github.com/miebyte/goutils/logging/level"

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
			return err
		}
	}

	return nil
}
