package prometheusutils

import (
	"github.com/miebyte/goutils/internal/buildinfo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var defaultRegistry *prometheus.Registry = CreateRegistry()

func CreateRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return reg
}

// GetCommonLabelsMap 获取公共标签
func GetCommonLabelsMap() map[string]string {
	return map[string]string{
		"server_name": buildinfo.GetServiceName(),
	}
}

func GetCommonLabelsMapWithModule(module ProModule) map[string]string {
	labels := GetCommonLabelsMap()
	labels["module"] = module.String()
	return labels
}

// CustomCounter 自定义的counter监控
var CustomCounter = promauto.With(defaultRegistry).NewCounterVec(
	prometheus.CounterOpts{
		Name:        "custom_counter_total",
		Help:        "custom counter monitor",
		ConstLabels: GetCommonLabelsMap(),
	},
	[]string{"module"},
)

// APICounter 接口访问数量的监控对象
var APICounter = promauto.With(defaultRegistry).NewCounter(
	prometheus.CounterOpts{
		Name:        "api_counter_total",
		Help:        "api counter monitor",
		ConstLabels: GetCommonLabelsMapWithModule(APIMonitor),
	},
)

// APIHistogram 接口访问耗时的监控对象
var APIHistogram = promauto.With(defaultRegistry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:        "api_histogram",
		Help:        "api histogram monitor",
		Buckets:     []float64{100, 500, 1000, 5000},
		ConstLabels: GetCommonLabelsMapWithModule(APIMonitor),
	},
	[]string{"url", "status_code"},
)

// TCPConnectionGauge 当前tcp连接数监控对象
var TCPConnectionGauge = promauto.With(defaultRegistry).NewGauge(
	prometheus.GaugeOpts{
		Name:        "tcp_connection_gauge",
		Help:        "tcp current connection count",
		ConstLabels: GetCommonLabelsMapWithModule(ServerMonitor),
	},
)

// RemoteConnectionGauge 远程连接数监控对象
var RemoteConnectionGauge = promauto.With(defaultRegistry).NewGauge(
	prometheus.GaugeOpts{
		Name:        "remote_connection_gauge",
		Help:        "remote current connection count",
		ConstLabels: GetCommonLabelsMapWithModule(ServerMonitor),
	},
)
