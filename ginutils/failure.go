// File:		failure.go
// Created by:	Hoven
// Created on:	2026-09-21
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package ginutils

import (
	"github.com/gin-gonic/gin"
)

// FailureStage 标明请求数据处理失败的阶段。
type FailureStage uint8

const (
	// FailureBind 表示请求数据绑定失败，例如参数无法解析成目标类型。
	FailureBind FailureStage = iota
	// FailureValidate 表示请求数据校验失败，例如必填项为空。
	FailureValidate
	// FailureModify 表示请求数据清洗失败，例如修饰器执行出错。
	FailureModify
)

// BindFailureHandler 把请求数据绑定、清洗或校验失败转换为响应错误。返回值交给
// ReturnError，因此返回实现了 ErrCoder 与 HTTPStatusCoder 的 error 可以同时
// 自定义业务码与 HTTP 状态码；返回字符串时业务码为 -1、HTTP 状态码为 200。
type BindFailureHandler func(c *gin.Context, stage FailureStage, err error) any

// bindFailureHandler 保存当前生效的失败响应构造器，nil 表示使用默认行为。
var bindFailureHandler BindFailureHandler

// SetBindFailureHandler 替换请求数据失败时的响应构造器，传入 nil 恢复默认行为。
// 建议在服务启动时设置一次，运行期间不要再修改。
func SetBindFailureHandler(handler BindFailureHandler) {
	bindFailureHandler = handler
}

// reportBindFailure 按当前构造器返回失败响应；未设置构造器时保留原始文案。
func reportBindFailure(c *gin.Context, stage FailureStage, err error) {
	if bindFailureHandler == nil {
		ReturnError(c, defaultBindFailureMessage(stage, err))
		return
	}
	ReturnError(c, bindFailureHandler(c, stage, err))
}

// defaultBindFailureMessage 返回默认的失败文案。
func defaultBindFailureMessage(stage FailureStage, err error) any {
	switch stage {
	case FailureValidate:
		return validateFailureMessage(err)
	case FailureModify:
		return "Failed to modify request data: " + err.Error()
	default:
		return "Failed to bind request data: " + err.Error()
	}
}
