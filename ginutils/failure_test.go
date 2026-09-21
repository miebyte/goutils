// File:		failure_test.go
// Created by:	Hoven
// Created on:	2026-09-21
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package ginutils

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type failureTestReq struct {
	Name string `form:"name" validate:"required"`
	Size int    `form:"size"`
}

// serveFailureRequest 用真实路由触发一次绑定或校验失败。
func serveFailureRequest(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/failures", RequestHandler(func(c *gin.Context, req *failureTestReq) {
		ReturnSuccess(c, req)
	}))
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	return w
}

// TestBindFailureKeepsDefaultBehaviour 未设置构造器时保留原始的文案与 200 状态码。
func TestBindFailureKeepsDefaultBehaviour(t *testing.T) {
	w := serveFailureRequest(t, "/failures?size=abc")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ret := decodeRet(t, w)
	if ret.Code != -1 {
		t.Fatalf("code = %v, want -1", ret.Code)
	}
	msg, _ := ret.Message.(string)
	if !strings.HasPrefix(msg, "Failed to bind request data: ") {
		t.Fatalf("message = %v", ret.Message)
	}
}

// TestValidateFailureKeepsDefaultBehaviour 校验失败同样保留翻译后的文案与 200 状态码。
func TestValidateFailureKeepsDefaultBehaviour(t *testing.T) {
	w := serveFailureRequest(t, "/failures?name=&size=1")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ret := decodeRet(t, w)
	if ret.Code != -1 {
		t.Fatalf("code = %v, want -1", ret.Code)
	}
	msg, _ := ret.Message.(string)
	if !strings.Contains(msg, "必填") {
		t.Fatalf("message = %v, want a translated required message", ret.Message)
	}
}

// TestSetBindFailureHandlerMapsFailure 自定义构造器可让失败响应带上业务码与 HTTP 状态码。
func TestSetBindFailureHandlerMapsFailure(t *testing.T) {
	stages := []FailureStage{}
	defer SetBindFailureHandler(nil)
	SetBindFailureHandler(func(_ *gin.Context, stage FailureStage, _ error) any {
		stages = append(stages, stage)
		return testFullCoder{code: 100400, status: http.StatusBadRequest, msg: "请求内容无效"}
	})

	w := serveFailureRequest(t, "/failures?size=abc")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bind status = %d, want 400", w.Code)
	}
	if ret := decodeRet(t, w); ret.Code != 100400 || ret.Message != "请求内容无效" {
		t.Fatalf("bind response = %+v", ret)
	}

	w = serveFailureRequest(t, "/failures?name=&size=1")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("validate status = %d, want 400", w.Code)
	}
	if ret := decodeRet(t, w); ret.Code != 100400 || ret.Message != "请求内容无效" {
		t.Fatalf("validate response = %+v", ret)
	}

	if len(stages) != 2 || stages[0] != FailureBind || stages[1] != FailureValidate {
		t.Fatalf("stages = %v, want [bind validate]", stages)
	}
}

// TestSetBindFailureHandlerNilRestoresDefault 传入 nil 恢复默认行为。
func TestSetBindFailureHandlerNilRestoresDefault(t *testing.T) {
	SetBindFailureHandler(func(c *gin.Context, stage FailureStage, err error) any {
		return "custom"
	})
	SetBindFailureHandler(nil)

	w := serveFailureRequest(t, "/failures?size=abc")
	ret := decodeRet(t, w)
	msg, _ := ret.Message.(string)
	if w.Code != http.StatusOK || ret.Code != -1 || !strings.HasPrefix(msg, "Failed to bind request data: ") {
		t.Fatalf("default behaviour was not restored: status=%d reply=%+v", w.Code, ret)
	}
}
