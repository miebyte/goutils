package prometheusutils

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/miebyte/goutils/logging"
)

// SendCustomCounter 发送自定义的counter监控
func SendCustomCounter(module string) {
	CustomCounter.WithLabelValues(module).Inc()
}

// SendAPICounter 发送所有接口访问数量的监控
func SendAPICounter() {
	APICounter.Inc()
}

// SendAPIHistogram 发送接口访问耗时的监控
func SendAPIHistogram(url string, processTime float64, statusCode int) {
	APIHistogram.WithLabelValues(url, strconv.Itoa(statusCode)).Observe(processTime)
}

var (
	bizCodeRegex = regexp.MustCompile(`(/)\d+(/|$|\?|#)`)
)

// SendCurrentTCPConnectionGauge 发送tcp连接数的监控
func SendCurrentTCPConnectionGauge() {
	// 获取当前tcp连接数
	content, err := os.ReadFile("/proc/1/net/snmp")
	if err != nil {
		logging.Errorf("read '/proc/1/net/snmp' error: %v", err)
		return
	}

	lines := strings.Split(string(content), "\n")
	var connectionCount float64

	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		typeName := strings.TrimSpace(parts[0])
		if strings.ToLower(typeName) == "tcp" {
			content := strings.TrimSpace(parts[1])
			countList := strings.Fields(content)
			if len(countList) >= 9 {
				if count, err := strconv.ParseFloat(countList[8], 64); err == nil {
					connectionCount = count
					break
				}
			}
		}
	}

	TCPConnectionGauge.Set(connectionCount)
}
