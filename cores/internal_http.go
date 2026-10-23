package cores

import "net/http"

const (
	defaultInternalHTTPAddress = ":6060"
	readyCheckUrl              = "/ready"
)

// WithInternalHTTPAddress 覆盖当前服务的管理监听地址，不改变业务端口。
// 默认监听所有网卡，便于 K8s 探针和 Consul 通过 Pod IP 检查当前实例。
// 本地同时启动多个服务时，应为每个服务指定不同的管理端口。
func WithInternalHTTPAddress(addr string) ServiceOption {
	return func(cs *CoresService) {
		cs.internalHTTP.addr = addr
	}
}

// readyCheckApi 只表达框架生命周期中的就绪状态。
// 业务依赖应在 Start 前完成初始化；开始停机后立即向调用方报告未就绪。
func (c *CoresService) readyCheckApi(w http.ResponseWriter, r *http.Request) {
	if !c.ready.Load() || c.stopping.Load() || c.ctx.Err() != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	healthCheckApi(w, r)
}
