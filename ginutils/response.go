// File:		response.go
// Created by:	Hoven
// Created on:	2025-06-05
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package ginutils

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	DefaultBindRequestFailedCode         = http.StatusOK
	DefaultValidateRequestDataFailedCode = http.StatusOK
	DefaultHandleResponseFailedCode      = http.StatusOK
)

const (
	successCode = 0
	failedCode  = -1
)

// Ret 是统一的 JSON 响应结构。
type Ret[T any] struct {
	Code    int `json:"code"`
	Data    T   `json:"data"`
	Message any `json:"message,omitempty"`
}

// SuccessRet 构造成功响应，业务码为 0。
func SuccessRet[T any](data T) *Ret[T] {
	return &Ret[T]{Code: successCode, Data: data, Message: "success"}
}

// ErrorRet 构造失败响应，业务码默认 -1，可通过 ErrCoder 自定义。
func ErrorRet(message any) *Ret[any] {
	var (
		msg  any
		code = failedCode
	)
	switch m := message.(type) {
	case error:
		coder, ok := errors.AsType[ErrCoder](m)
		if ok {
			code = coder.Code()
		}

		msg = m.Error()
	case string:
		msg = m
	default:
		msg = m
	}

	return &Ret[any]{Code: code, Data: nil, Message: msg}
}

// ReturnSuccess 返回成功响应，HTTP 状态码为 200。
func ReturnSuccess(c *gin.Context, data any) {
	c.JSON(http.StatusOK, SuccessRet(data))
}

// ReturnError 返回失败响应，HTTP 状态码默认 200。
func ReturnError(c *gin.Context, message any) {
	c.JSON(resolveHTTPStatus(message), ErrorRet(message))
}

// ReturnErrorWithStatus 以指定 HTTP 状态码返回失败响应。
func ReturnErrorWithStatus(c *gin.Context, status int, message any) {
	c.JSON(status, ErrorRet(message))
}

// ReturnErrorWithCode 以指定 HTTP 状态码返回失败响应。
func ReturnErrorWithCode(c *gin.Context, code int, message any) {
	ReturnErrorWithStatus(c, code, message)
}
