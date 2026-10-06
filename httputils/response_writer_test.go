package httputils

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewStatusCodeResponseWriter(t *testing.T) {
	// 创建一个测试用的 ResponseWriter
	recorder := httptest.NewRecorder()

	// 创建 StatusCodeResponseWriter
	writer := NewStatusCodeResponseWriter(recorder)

	// 验证默认状态码
	if writer.StatusCode != http.StatusOK {
		t.Errorf("期望默认状态码为 %d，实际为 %d", http.StatusOK, writer.StatusCode)
	}

	// 验证 ResponseWriter 被正确设置
	if writer.ResponseWriter == nil {
		t.Error("ResponseWriter 不应该为 nil")
	}
}

func TestStatusCodeResponseWriter_WriteHeader(t *testing.T) {
	// 创建测试用例
	testCases := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"201 Created", http.StatusCreated},
		{"400 Bad Request", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建测试用的 ResponseWriter
			recorder := httptest.NewRecorder()

			// 创建 StatusCodeResponseWriter
			writer := NewStatusCodeResponseWriter(recorder)

			// 调用 WriteHeader
			writer.WriteHeader(tc.statusCode)

			// 验证状态码被正确设置
			if writer.StatusCode != tc.statusCode {
				t.Errorf("期望状态码为 %d，实际为 %d", tc.statusCode, writer.StatusCode)
			}

			// 验证底层 ResponseWriter 也收到了正确的状态码
			if recorder.Code != tc.statusCode {
				t.Errorf("底层 ResponseWriter 状态码不匹配，期望 %d，实际 %d", tc.statusCode, recorder.Code)
			}
		})
	}
}

func TestStatusCodeResponseWriter_Write(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := NewStatusCodeResponseWriter(recorder)

	// 测试写入数据
	testData := []byte("Hello, World!")
	written, err := writer.Write(testData)

	// 验证写入成功
	if err != nil {
		t.Errorf("写入数据时发生错误: %v", err)
	}

	if written != len(testData) {
		t.Errorf("期望写入 %d 字节，实际写入 %d 字节", len(testData), written)
	}

	// 验证数据被正确写入底层 ResponseWriter
	if recorder.Body.String() != string(testData) {
		t.Errorf("期望响应体为 %s，实际为 %s", string(testData), recorder.Body.String())
	}
}

func TestStatusCodeResponseWriter_Header(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := NewStatusCodeResponseWriter(recorder)

	// 测试设置响应头
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("X-Custom-Header", "test-value")

	// 验证响应头被正确设置
	if recorder.Header().Get("Content-Type") != "application/json" {
		t.Error("Content-Type 响应头设置失败")
	}

	if recorder.Header().Get("X-Custom-Header") != "test-value" {
		t.Error("X-Custom-Header 响应头设置失败")
	}
}
