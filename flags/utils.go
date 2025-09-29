package flags

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/spf13/cast"
)

// toCaseInsensitiveValue checks if the value is a  map;
// if so, create a copy and lower-case the keys recursively.
func toCaseInsensitiveValue(value any) any {
	switch v := value.(type) {
	case map[any]any:
		value = copyAndInsensitiviseMap(cast.ToStringMap(v))
	case map[string]any:
		value = copyAndInsensitiviseMap(v)
	}

	return value
}

// copyAndInsensitiviseMap behaves like insensitiviseMap, but creates a copy of
// any map it makes case insensitive.
func copyAndInsensitiviseMap(m map[string]any) map[string]any {
	nm := make(map[string]any)

	for key, val := range m {
		lkey := strings.ToLower(key)
		switch v := val.(type) {
		case map[any]any:
			nm[lkey] = copyAndInsensitiviseMap(cast.ToStringMap(v))
		case map[string]any:
			nm[lkey] = copyAndInsensitiviseMap(v)
		default:
			nm[lkey] = v
		}
	}

	return nm
}

// CopyAndInsensitiviseMapPublic exports copyAndInsensitiviseMap for other packages.
func CopyAndInsensitiviseMapPublic(m map[string]any) map[string]any {
	return copyAndInsensitiviseMap(m)
}

func absPathify(inPath string) string {
	if inPath == "$HOME" || strings.HasPrefix(inPath, "$HOME"+string(os.PathSeparator)) {
		inPath = userHomeDir() + inPath[5:]
	}

	inPath = os.ExpandEnv(inPath)

	if filepath.IsAbs(inPath) {
		return filepath.Clean(inPath)
	}

	p, err := filepath.Abs(inPath)
	if err == nil {
		return filepath.Clean(p)
	}

	innerlog.Logger.Error(fmt.Errorf("could not discover absolute path: %w", err).Error())

	return ""
}

func userHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}
