// File:		handler.go
// Created by:	Hoven
// Created on:	2025-06-05
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package ginutils

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/utils/reflectx"
)

type requestHandler[Q any] func(c *gin.Context, req *Q)

func isValidHTTPStatusCode(code int) bool {
	return code >= 100 && code < 600 && http.StatusText(code) != ""
}

func RequestHandler[Q any](fn requestHandler[Q]) gin.HandlerFunc {
	reqType := reflect.TypeOf((*Q)(nil)).Elem()
	reqKind := reflectx.ResolveBaseKind(reqType)
	reqStrategies := resolveStrategies(reqType)

	return func(c *gin.Context) {
		ctx := c.Request.Context()
		reqPtr := new(Q)

		if err := bindRequestData(c, reqPtr, reqStrategies); err != nil {
			logging.Errorc(ctx, "failed to bind request data %T. error: %v", reqPtr, err)
			ReturnError(c, "Failed to bind request data: "+err.Error())
			return
		}

		// 数据清洗, 忽略错误
		modifyErr := modifyRequestData(ctx, reqPtr, reqKind)
		if modifyErr != nil {
			logging.Errorc(ctx, "failed to modify reqPtr(%T). error: %v", reqPtr, modifyErr)
		}

		// 数据校验
		validateErr := validateRequestData(ctx, reqPtr, reqKind)
		if validateErr != nil {
			handleValidateError(c, validateErr)
			return
		}

		fn(c, reqPtr)
	}
}

type requestResponseHandler[Q any, P any] func(c *gin.Context, req *Q) (resp *P, err error)

func RequestResponseHandler[Q any, P any](fn requestResponseHandler[Q, P]) gin.HandlerFunc {
	reqType := reflect.TypeOf((*Q)(nil)).Elem()
	reqKind := reflectx.ResolveBaseKind(reqType)
	reqStrategies := resolveStrategies(reqType)

	return func(c *gin.Context) {
		ctx := c.Request.Context()
		reqPtr := new(Q)

		if err := bindRequestData(c, reqPtr, reqStrategies); err != nil {
			logging.Errorc(ctx, "failed to bind request data %T. error: %v", reqPtr, err)
			ReturnError(c, "Failed to bind request data: "+err.Error())
			return
		}

		// 数据清洗, 忽略错误
		modifyErr := modifyRequestData(ctx, reqPtr, reqKind)
		if modifyErr != nil {
			logging.Errorc(ctx, "failed to modify reqPtr(%T). error: %v", reqPtr, modifyErr)
		}

		// 数据校验
		validateErr := validateRequestData(ctx, reqPtr, reqKind)
		if validateErr != nil {
			handleValidateError(c, validateErr)
			return
		}

		resp, err := fn(c, reqPtr)
		if err != nil {
			handleError(c, err)
			return
		}

		if resp == nil {
			return
		}

		c.JSON(http.StatusOK, SuccessRet(resp))
	}
}

func handleError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	ReturnError(c, err)
	innerlog.Logger.Errorc(c.Request.Context(), "handle request: %s error: %v", c.Request.URL.Path, err)
}

func prepareMount[T any]() error {
	// r is a pointer of ModelHandler like *ModelHandler
	r := new(T)
	to := reflect.TypeOf(r)

	// if depth == 1, it means MountHandler[MountTestHandler]()
	// if depth == 2, it means MountHandler[*MountTestHandler]()
	depth := 0

	for to.Kind() == reflect.Pointer {
		to = to.Elem()
		depth++
	}

	if depth != 1 {
		return fmt.Errorf("depth not equal to 1")
	}

	return nil
}

type ModelHandler[R any] interface {
	Handle(c *gin.Context) (resp *R, err error)
}

func MountHandler[MH ModelHandler[R], R any]() gin.HandlerFunc {
	err := prepareMount[MH]()
	if err != nil {
		panic("MountHandler[]() Generic types should not be pointer types")
	}

	return RequestResponseHandler(func(c *gin.Context, req *MH) (resp *R, err error) {
		resp, err = (*req).Handle(c)
		return
	})
}

type ModelWithArgHandler[R any, ARG any] interface {
	Handle(c *gin.Context, arg ARG) (resp *R, err error)
}

func MountHandlerWithArg[MH ModelWithArgHandler[R, ARG], R any, ARG any](arg ARG) gin.HandlerFunc {
	err := prepareMount[MH]()
	if err != nil {
		panic("MountHandlerWithArg[]() Generic types should not be pointer types")
	}

	return RequestResponseHandler(func(c *gin.Context, req *MH) (resp *R, err error) {
		resp, err = (*req).Handle(c, arg)
		return
	})
}
