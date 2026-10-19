package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/miebyte/goutils/internal/innerlog"
)

var (
	configUp = 0
)

func SetConfigUp(up int) {
	configUp = up
}

// resolveRootPath 从cwd 向上回退 up 层，得到额外的 superconf.json 候选目录。
func resolveRootPath(cwd string, up int) string {
	if up < 0 {
		up = 0
	}
	for i := 0; i < up; i++ {
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return cwd
}

// GetConfig 获取配置文件
// name 配置文件名
func GetConfig[T any](name string) (T, error) {
	var zero T

	configFileName := fmt.Sprintf("%s.json", name)

	cwd, err := os.Getwd()
	if err != nil {
		return zero, fmt.Errorf("获取当前工作目录失败: %v", err)
	}

	rootPaths := []string{cwd}
	if rootPath := resolveRootPath(cwd, configUp); rootPath != cwd {
		rootPaths = append(rootPaths, rootPath)
	}

	var (
		filePath    string
		fileContent []byte
	)
	for _, rootPath := range rootPaths {
		filePath = filepath.Join(rootPath, configFileName)
		fileContent, err = os.ReadFile(filePath)
		if err == nil {
			break
		}

		// 除了文件不存在，其他错误都返回
		if !os.IsNotExist(err) {
			return zero, fmt.Errorf("读取 %s 配置失败: %v", configFileName, err)
		}
	}

	if os.IsNotExist(err) {
		return zero, fmt.Errorf("%s 配置文件不存在", configFileName)
	} else if err != nil {
		return zero, fmt.Errorf("读取%s配置失败: %v", configFileName, err)
	}
	innerlog.Logger.Infof("read %s from: %s", name, filePath)

	var config T
	err = json.Unmarshal(fileContent, &config)
	if err != nil {
		return zero, fmt.Errorf("解析%s配置失败: %v", configFileName, err)
	}

	return config, nil
}
