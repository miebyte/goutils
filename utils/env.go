package utils

import (
	"os"
)

func GetEnvByDefualt(key, defaultVal string) string {
	val := os.Getenv(key)
	if val != "" {
		return val
	}

	return defaultVal
}
