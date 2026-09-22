package ginutils

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type testErrCoder struct {
	code int
	msg  string
}

func (e testErrCoder) Error() string { return e.msg }
func (e testErrCoder) Code() int     { return e.code }

type testStatusCoder struct {
	msg    string
	status int
}

func (e testStatusCoder) Error() string   { return e.msg }
func (e testStatusCoder) HTTPStatus() int { return e.status }

type testFullCoder struct {
	code   int
	status int
	msg    string
}

func (e testFullCoder) Error() string   { return e.msg }
func (e testFullCoder) Code() int       { return e.code }
func (e testFullCoder) HTTPStatus() int { return e.status }

func newTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return c, w
}

func decodeRet(t *testing.T, w *httptest.ResponseRecorder) Ret[any] {
	t.Helper()
	var ret Ret[any]
	if err := json.Unmarshal(w.Body.Bytes(), &ret); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, w.Body.String())
	}
	return ret
}

func TestReturnSuccess(t *testing.T) {
	c, w := newTestContext()
	ReturnSuccess(c, "ok")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ret := decodeRet(t, w)
	if ret.Code != 0 {
		t.Fatalf("code = %d, want 0", ret.Code)
	}
	if ret.Data != "ok" {
		t.Fatalf("data = %v, want ok", ret.Data)
	}
}

func TestReturnErrorDefault(t *testing.T) {
	c, w := newTestContext()
	ReturnError(c, "failed")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ret := decodeRet(t, w)
	if ret.Code != -1 {
		t.Fatalf("code = %d, want -1", ret.Code)
	}
	if ret.Message != "failed" {
		t.Fatalf("message = %v, want failed", ret.Message)
	}
}

func TestReturnErrorWithErrCoder(t *testing.T) {
	c, w := newTestContext()
	ReturnError(c, testErrCoder{code: 1001, msg: "biz error"})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	ret := decodeRet(t, w)
	if ret.Code != 1001 {
		t.Fatalf("code = %d, want 1001", ret.Code)
	}
	if ret.Message != "biz error" {
		t.Fatalf("message = %v, want biz error", ret.Message)
	}
}

func TestReturnErrorWithHTTPStatusCoder(t *testing.T) {
	c, w := newTestContext()
	ReturnError(c, testStatusCoder{msg: "unauthorized", status: http.StatusUnauthorized})

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	ret := decodeRet(t, w)
	if ret.Code != -1 {
		t.Fatalf("code = %d, want -1", ret.Code)
	}
}

func TestReturnErrorWithFullCoder(t *testing.T) {
	c, w := newTestContext()
	ReturnError(c, testFullCoder{code: 1002, status: http.StatusForbidden, msg: "forbidden"})

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	ret := decodeRet(t, w)
	if ret.Code != 1002 {
		t.Fatalf("code = %d, want 1002", ret.Code)
	}
}

func TestReturnErrorWithStatus(t *testing.T) {
	c, w := newTestContext()
	ReturnErrorWithStatus(c, http.StatusBadRequest, "bad request")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	ret := decodeRet(t, w)
	if ret.Code != -1 {
		t.Fatalf("code = %d, want -1", ret.Code)
	}
}

func TestWithHTTPStatus(t *testing.T) {
	c, w := newTestContext()
	err := WithHTTPStatus(errors.New("gone"), http.StatusGone)
	ReturnError(c, err)

	if w.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410", w.Code)
	}
	ret := decodeRet(t, w)
	if ret.Code != -1 {
		t.Fatalf("code = %d, want -1", ret.Code)
	}
	if ret.Message != "gone" {
		t.Fatalf("message = %v, want gone", ret.Message)
	}
}

func TestWithHTTPStatusWrapsErrCoder(t *testing.T) {
	c, w := newTestContext()
	err := WithHTTPStatus(testErrCoder{code: 2001, msg: "wrapped"}, http.StatusConflict)
	ReturnError(c, fmt.Errorf("outer: %w", err))

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
	ret := decodeRet(t, w)
	if ret.Code != 2001 {
		t.Fatalf("code = %d, want 2001", ret.Code)
	}
}

func TestSetDefaultErrorHTTPStatus(t *testing.T) {
	SetDefaultErrorHTTPStatus(http.StatusInternalServerError)
	defer SetDefaultErrorHTTPStatus(DefaultErrorHTTPStatus)

	c, w := newTestContext()
	ReturnError(c, "failed")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestSetDefaultErrorHTTPStatusRecoversOnNonPositive(t *testing.T) {
	SetDefaultErrorHTTPStatus(http.StatusInternalServerError)
	SetDefaultErrorHTTPStatus(0)
	defer SetDefaultErrorHTTPStatus(DefaultErrorHTTPStatus)

	if errorHTTPStatus != DefaultErrorHTTPStatus {
		t.Fatalf("default status = %d, want %d", errorHTTPStatus, DefaultErrorHTTPStatus)
	}

	c, w := newTestContext()
	ReturnError(c, "failed")

	if w.Code != DefaultErrorHTTPStatus {
		t.Fatalf("status = %d, want %d", w.Code, DefaultErrorHTTPStatus)
	}
}

func TestSetDefaultErrorHTTPStatusKeepsExplicitStatus(t *testing.T) {
	SetDefaultErrorHTTPStatus(http.StatusInternalServerError)
	defer SetDefaultErrorHTTPStatus(DefaultErrorHTTPStatus)

	c, w := newTestContext()
	ReturnError(c, WithHTTPStatus(errors.New("gone"), http.StatusGone))

	if w.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410", w.Code)
	}
}
