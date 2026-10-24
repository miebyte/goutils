package main

import (
	"context"
	"flag"
	"time"

	"github.com/miebyte/goutils/consulutils"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/share"
)

var (
	target    = flag.String("target", "orders", "目标逻辑服务名")
	namespace = flag.String("namespace", "", "可选的 Kubernetes 命名空间；空值表示不限命名空间")
)

func main() {
	share.SetServiceName("consul-discovery-example")
	flag.Parse()

	// 不传配置时复用公共 Client，查询默认不限 namespace，也不读取 POD_NAMESPACE。
	var configs []consulutils.DiscoveryConfig
	if *namespace != "" {
		configs = append(configs, consulutils.DiscoveryConfig{Namespace: *namespace})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	addresses, err := consulutils.Discover(ctx, *target, configs...)
	logging.PanicError(err)
	logging.Infof("服务 %s 的健康地址: %v", *target, addresses)

	// Resolve 会重新查询，并从去重后的健康地址中随机选择一个。
	address, err := consulutils.Resolve(ctx, *target, configs...)
	logging.PanicError(err)
	// 此处仅输出客户端可使用的目标，不实际发起 HTTP 或 gRPC 业务请求。
	logging.Infof("HTTP 目标: http://%s", address)
	logging.Infof("gRPC 目标: %s", address)
}
