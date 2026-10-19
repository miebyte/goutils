package config

import (
	"os"
	"path/filepath"
)

// LocalConfigPath 定位本地业务配置，显式路径优先，否则依次查找当前目录、configs 和 HOME 中的 config.json。
// 与 GetConfig 读取远程连接引导配置不同，这里不使用 configUp，也不读取或缓存文件内容。
func LocalConfigPath(explicit string) string {
	if explicit != "" {
		return explicit
	}

	paths := []string{"config.json", filepath.Join("configs", "config.json")}
	if home := os.Getenv("HOME"); home != "" {
		paths = append(paths, filepath.Join(home, "config.json"))
	}
	for _, path := range paths {
		path, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
