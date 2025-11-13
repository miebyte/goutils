package prometheusutils

// ProModule 监控模块枚举
type ProModule string

const (
	APIMonitor    ProModule = "api_monitor"
	ServerMonitor ProModule = "server_monitor"
)

func (p ProModule) String() string {
	return string(p)
}
