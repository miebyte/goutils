package httputils

import "net/http"

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
