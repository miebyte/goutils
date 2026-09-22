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

// DefaultErrorHTTPStatus 是失败响应无法解析出自定义 HTTP 状态码时使用的默认值。
const DefaultErrorHTTPStatus = http.StatusOK

// errorHTTPStatus 保存当前生效的失败响应默认 HTTP 状态码。
var errorHTTPStatus = DefaultErrorHTTPStatus

// SetDefaultErrorHTTPStatus 设置失败响应无法解析出自定义 HTTP 状态码时使用的默认值，
// 传入非正数时恢复为 DefaultErrorHTTPStatus。建议在服务启动时设置一次，运行期间不要再修改。
func SetDefaultErrorHTTPStatus(status int) {
	if status <= 0 {
		status = DefaultErrorHTTPStatus
	}
	errorHTTPStatus = status
}

// resolveErrorResponseStatus 解析错误响应使用的 HTTP 状态码，无法解析时返回 errorHTTPStatus。
func resolveErrorResponseStatus(message any) int {
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

	return errorHTTPStatus
}
