package cores

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/prometheusutils"
)

const (
	metricsUrl = "/metrics"
)

// WithPrometheus 是否开启 prometheus 打点
func WithPrometheus() ServiceOption {
	return func(cs *CoresService) {
		cs.usePrometheus = true
	}
}

func (c *CoresService) setupPrometheus() {
	if !c.usePrometheus {
		return
	}

	if c.listenAddr == "" {
		innerlog.Logger.Warnf("Cores server not start by cores.Start(). Prometheus can not be enabled")
		c.usePrometheus = false
		return
	}

	c.httpMux.Handle(metricsUrl, prometheusutils.PrometheusHttpHandler())

	_, port, _ := net.SplitHostPort(c.listenAddr)
	target := fmt.Sprintf("localhost:%s", port)
	innerlog.Logger.Debugf("Prometheus enabled. URL=%s", fmt.Sprintf("http://%s%s", target, metricsUrl))
}

func (c *CoresService) monitorHttp(handler http.Handler) http.Handler {
	if !c.usePrometheus {
		return handler
	}

	ignoreUrls := []string{metricsUrl, pprofUrl, healthCheckUrl}
	checkIgnoreUrl := func(path string) bool {
		for _, ignoreUrl := range ignoreUrls {
			if strings.HasPrefix(path, ignoreUrl) {
				return true
			}
		}
		return false
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if checkIgnoreUrl(path) {
			handler.ServeHTTP(w, r)
			return
		}

		prometheusutils.SendAPICounter()

		start := time.Now()
		sw := NewStatusCodeResponseWriter(w)
		handler.ServeHTTP(sw, r)
		duration := time.Since(start).Milliseconds()

		prometheusutils.SendAPIHistogram(r.URL.Path, float64(duration), sw.StatusCode)
	})
}

type StatusCodeResponseWriter struct {
	http.ResponseWriter
	StatusCode int
}

func NewStatusCodeResponseWriter(w http.ResponseWriter) *StatusCodeResponseWriter {
	return &StatusCodeResponseWriter{
		ResponseWriter: w,
		StatusCode:     http.StatusOK,
	}
}

func (c *StatusCodeResponseWriter) WriteHeader(statusCode int) {
	c.StatusCode = statusCode
	c.ResponseWriter.WriteHeader(statusCode)
}
