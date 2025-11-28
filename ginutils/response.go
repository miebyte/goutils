// File:		response.go
// Created by:	Hoven
// Created on:	2025-06-05
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package ginutils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	successCode = 0
	failedCode  = -1
)

type Ret[T any] struct {
	Code    int `json:"code"`
	Data    T   `json:"data,omitempty"`
	Message any `json:"message,omitempty"`
}

func SuccessRet[T any](data T) *Ret[T] {
	return &Ret[T]{Code: successCode, Data: data, Message: "success"}
}

func ErrorRet(message any) *Ret[any] {
	var (
		msg  any
		code = failedCode
	)
	switch m := message.(type) {
	case ErrCoder:
		code = m.Code()
		msg = m.Error()
	case string:
		msg = m
	case error:
		msg = m.Error()
	default:
		msg = m
	}

	return &Ret[any]{Code: code, Data: nil, Message: msg}
}

func ReturnSuccess(c *gin.Context, data any) {
	c.JSON(http.StatusOK, SuccessRet(data))
}

func ReturnError(c *gin.Context, message any) {
	c.JSON(http.StatusOK, ErrorRet(message))
}
