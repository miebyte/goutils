package provider

import (
	"context"
	"encoding/json"
	"errors"
)

// ErrConfigNotFound 表示配置确实不存在，首次加载时可使用声明的默认值。
// 连接、权限、取消和解析错误不能作为此错误返回。
var ErrConfigNotFound = errors.New("config not found")

// Event 携带配置路径及完整原文，解析和校验由使用方负责。
type Event struct {
	Path   string
	Config json.RawMessage
	Err    error
}

type ConfigReader interface {
	// ReadConfig 返回配置原文
	ReadConfig(context.Context, string) (json.RawMessage, error)
}

type ConfigWatcher interface {
	// WatchConfig 仅发送订阅后的配置变更，不发送初始快照；初值通过 ReadConfig 读取。
	// 监听生命周期由 provider 管理。
	// 事件携带完整配置，可合并积压，但不能使较旧快照覆盖较新快照。
	WatchConfig(string) (<-chan Event, error)
}

type ConfigProvider interface {
	ConfigReader
	ConfigWatcher
	Path(string) string
	// Close 结束当前 provider 的监听并关闭事件通道，重复调用安全。
	// 使用公共 client 时不关闭连接池，避免影响服务注册等其他使用方。
	Close() error
}
