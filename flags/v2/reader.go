package flags

import (
	"context"
	"encoding/json"

	"github.com/miebyte/goutils/flags/v2/internal/local"
)

type keyReader struct {
	key        string
	isFullPath bool
	path       string

	provider    ConfigProvider
	isStruct    bool // 只有 Struct 的配置要求 JSON，普通 Flag 可以是纯文本。
	confRefresh func(json.RawMessage) error
}

// ReadConfig 将本地 key 映射到远程路径；
// 本地模式始终按 key 读取，不使用远程覆盖选项。
func (k keyReader) ReadConfig(ctx context.Context, defaultProvider ConfigProvider) (json.RawMessage, error) {
	source, path, err := k.resolve(defaultProvider)
	if err != nil {
		return nil, err
	}
	return source.ReadConfig(ctx, path)
}

// resolve 解析当前keyReader 的读取配置应该使用的配置源和路径。
func (k keyReader) resolve(defaultProvider ConfigProvider) (ConfigProvider, string, error) {
	if _, localMode := defaultProvider.(*local.LocalProvider); localMode {
		return defaultProvider, k.key, nil
	}

	source := defaultProvider
	if k.provider != nil {
		source = k.provider
	}

	if k.isFullPath {
		return source, k.path, nil
	}

	path, err := configPath(source, k.key, k.path)
	if err != nil {
		return nil, "", err
	}

	return source, path, nil
}
