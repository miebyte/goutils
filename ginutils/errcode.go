package ginutils

import (
	"errors"
	"net/http"
)

// ErrCoder 提供自定义业务错误码。
type ErrCoder interface {
	error
	Code() int
}

// HTTPStatusCoder 提供自定义 HTTP 状态码。
type HTTPStatusCoder interface {
	HTTPStatus() int
}

// statusError 为底层错误附加 HTTP 状态码。
type statusError struct {
	err    error
	status int
}

// Error 返回被包装错误的文案。
func (e *statusError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

// Unwrap 返回被包装的原始错误。
func (e *statusError) Unwrap() error {
	return e.err
}

// HTTPStatus 返回附加的 HTTP 状态码。
func (e *statusError) HTTPStatus() int {
	return e.status
}

// WithHTTPStatus 将错误包装为带自定义 HTTP 状态码的错误。
func WithHTTPStatus(err error, status int) error {
	if err == nil {
		return nil
	}
	return &statusError{err: err, status: status}
}

// resolveHTTPStatus 解析响应使用的 HTTP 状态码，默认 200。
func resolveHTTPStatus(message any) int {
	if coder, ok := message.(HTTPStatusCoder); ok {
		if status := coder.HTTPStatus(); status > 0 {
			return status
		}
	}

	if err, ok := message.(error); ok {
		var coder HTTPStatusCoder
		if errors.As(err, &coder) {
			if status := coder.HTTPStatus(); status > 0 {
				return status
			}
		}
	}

	return http.StatusOK
}
