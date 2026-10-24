// Package consulutils 提供配置读取、服务注册和服务发现共用的 Consul 能力。
package consulutils

import (
	"fmt"
	"sync"

	consulapi "github.com/hashicorp/consul/api"
)

// 只在第一次使用时读取 SDK 默认配置和 CONSUL_* 环境变量。
// 构造失败也缓存结果，避免错误配置导致每次访问都重复创建连接池。
var defaultClient = sync.OnceValues(func() (*consulapi.Client, error) {
	client, err := consulapi.NewClient(consulapi.DefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("create consul client: %w", err)
	}
	return client, nil
})

// DefaultClient 返回进程内共享的 Consul client，首次调用前应准备好环境变量。
// 创建 client 不连接 Consul；请求超时由调用方的 context 控制，避免截断 KV 长轮询。
// 配置监听、服务注册和服务发现只管理各自任务，不能关闭共享 client 的连接池。
func DefaultClient() (*consulapi.Client, error) {
	return defaultClient()
}
