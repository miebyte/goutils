package httputils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	server *httptest.Server
)

type mockResponse struct {
	Headers http.Header
	Params  map[string]string
	Url     string
	Method  string
	Body    map[string]any
	Err     string
}

func TestMain(m *testing.M) {
	// mock http server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &mockResponse{}
		resp.Url = r.URL.String()
		resp.Method = r.Method
		resp.Body = map[string]any{}
		resp.Params = map[string]string{}
		resp.Headers = r.Header

		// 解析查询参数
		queryParams := r.URL.Query()
		for key := range queryParams {
			resp.Params[key] = queryParams.Get(key)
		}

		// 解析请求体
		if r.Body != nil {
			b, err := io.ReadAll(r.Body)
			if err == nil && len(b) > 0 {
				err = json.Unmarshal(b, &resp.Body)
				if err != nil {
					fmt.Println("unmarshal body error", err)
					resp.Err = err.Error()
				}
			} else if err != nil {
				fmt.Println("read body error", err)
				resp.Err = err.Error()
			}
		}

		time.Sleep(100 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			fmt.Fprintf(w, "Failed to encode response: %v", err)
			return
		}
	}))
	defer server.Close()

	m.Run()
}

func TestBasicUsage(t *testing.T) {
	client := NewClient()

	// 发送GET请求
	resp := client.Start().
		SetURL(server.URL).
		SetHeader("User-Agent", "go-utils/1.0").
		Get(context.Background())

	// 检查错误
	if resp.Error() != nil {
		t.Errorf("Request failed: %v", resp.Error())
		return
	}

	// 检查响应状态
	if !resp.IsSuccess() {
		t.Errorf("Request failed with status: %d", resp.StatusCode())
		return
	}

	// 获取响应体字符串
	body, err := resp.BodyString()
	if err != nil {
		t.Errorf("Failed to read response body: %v", err)
		return
	}

	t.Logf("Response: %s", body)
}

// TestRequestContextTimeout 验证请求上下文超时会取消请求
func TestRequestContextTimeout(t *testing.T) {
	timeoutCh := make(chan time.Duration, 1)
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		<-r.Context().Done()
		timeoutCh <- time.Since(start)
	}))
	defer slowServer.Close()

	client := NewClient()
	resp := client.Start().
		SetURL(slowServer.URL).
		SetTimeout(30 * time.Millisecond).
		Get(context.Background())

	t.Logf("resp: %v", resp.Error())
	if respErr := resp.Error(); respErr == nil {
		t.Fatal("expected timeout error")
	}

	select {
	case duration := <-timeoutCh:
		if duration > 80*time.Millisecond {
			t.Fatalf("timeout not respected, request took %v", duration)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("request context timeout was not triggered")
	}
}

func TestJsonResponse(t *testing.T) {
	client := NewClient()

	// 发送请求
	resp := client.Start().
		SetURL(server.URL).
		SetHeader("Content-Type", "application/json").
		Get(context.Background())

	if resp.Error() != nil {
		t.Errorf("Request failed: %v", resp.Error())
		return
	}
	b, err := resp.BodyBytes()
	if err != nil {
		t.Errorf("Failed to read response body: %v", err)
		return
	}

	t.Logf("result: %s", string(b))
}

func TestPostRequest(t *testing.T) {
	client := NewClient()

	// 使用 JsonBody 构建请求数据
	jsonBody := NewJsonBody().
		Add("name", "test").
		Add("value", 123)

	resp := client.Start().
		SetURL(server.URL).
		SetJSONBody(jsonBody).
		Post(context.Background())

	if resp.Error() != nil {
		t.Errorf("Request failed: %v", resp.Error())
		return
	}

	bodyBytes, err := resp.BodyBytes()
	if err != nil {
		t.Errorf("Failed to read response body: %v", err)
		return
	}

	t.Logf("Response: %s", string(bodyBytes))
}

func TestWithHeaders(t *testing.T) {
	client := NewClient()

	resp := client.Start().
		SetURL(server.URL).
		SetHeader("User-Agent", "go-utils/2.0").
		SetHeader("Accept", "application/json").
		SetHeaders(map[string]string{
			"Custom-Header": "custom-value",
		}).
		Get(context.Background())

	if resp.Error() != nil {
		t.Errorf("Request failed: %v", resp.Error())
		return
	}

	b, err := resp.BodyBytes()
	if err != nil {
		t.Errorf("Failed to read response body: %v", err)
		return
	}

	t.Logf("Response: %s", string(b))
}

func TestWithParams(t *testing.T) {
	client := NewClient()

	// 使用 Params 构建查询参数
	params := NewParams().
		Add("page", "1").
		Add("limit", "10")

	resp := client.Start().
		SetURL(server.URL).
		SetQueryParams(params).
		SetQueryParam("sort", "desc").
		Get(context.Background())

	if resp.Error() != nil {
		t.Errorf("Request failed: %v", resp.Error())
		return
	}

	t.Logf("Status: %d", resp.StatusCode())
}

func TestWithMiddleware(t *testing.T) {
	client := NewClient().
		Use(HandlerDuration())

	resp := client.Start().
		SetURL(server.URL).
		SetTimeout(10 * time.Second).
		Get(context.Background())

	if resp.Error() != nil {
		t.Errorf("Request failed: %v", resp.Error())
		return
	}

	t.Logf("Status: %d", resp.StatusCode())
}

func TestResponse_Methods(t *testing.T) {
	// 测试错误响应
	errorResp := NewResponse(nil, fmt.Errorf("test error"))
	if errorResp.Error() == nil {
		t.Error("Expected error, got nil")
	}

	if errorResp.IsSuccess() {
		t.Error("Error response should not be success")
	}

	if errorResp.StatusCode() != 0 {
		t.Errorf("Expected status code 0, got %d", errorResp.StatusCode())
	}

	// 测试BodyString错误情况
	_, err := errorResp.BodyString()
	if err == nil {
		t.Error("Expected error when getting body string from error response")
	}

	// 测试BodyBytes错误情况
	_, err = errorResp.BodyBytes()
	if err == nil {
		t.Error("Expected error when getting body bytes from error response")
	}

	// 测试BodyJson错误情况
	var data map[string]interface{}
	err = errorResp.BodyJson(&data)
	if err == nil {
		t.Error("Expected error when parsing JSON from error response")
	}
}

// TestResponse_JsonValidation 测试JSON解析验证
func TestResponse_JsonValidation(t *testing.T) {
	resp := NewResponse(nil, nil)

	// 测试nil值
	err := resp.BodyJson(nil)
	if err == nil || err.Error() != "[BodyJson] value is nil" {
		t.Errorf("Expected '[BodyJson] value is nil' error, got: %v", err)
	}

	// 测试非指针值
	var data map[string]any
	err = resp.BodyJson(data)
	if err == nil || err.Error() != "[BodyJson] value is not ptr" {
		t.Errorf("Expected '[BodyJson] value is not ptr' error, got: %v", err)
	}
}

func TestResponse_DecodeJSON(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	resp := NewResponse(&http.Response{
		Body: io.NopCloser(strings.NewReader(`{"name":"codex","count":2}`)),
	}, nil)

	result, err := resp.DecodeJSON[payload]()
	if err != nil {
		t.Fatalf("DecodeJSON() error = %v", err)
	}
	if result.Name != "codex" || result.Count != 2 {
		t.Fatalf("DecodeJSON() = %+v", result)
	}
}

func TestResponse_DecodeJSONError(t *testing.T) {
	resp := NewResponse(&http.Response{
		Body: io.NopCloser(strings.NewReader(`{"name":`)),
	}, nil)

	result, err := resp.DecodeJSON[map[string]any]()
	if err == nil {
		t.Fatal("DecodeJSON() expected an error")
	}
	if result != nil {
		t.Fatalf("DecodeJSON() result = %v, want nil", result)
	}
}

// BenchmarkResponse_BodyBytes 性能测试
func BenchmarkResponse_BodyBytes(b *testing.B) {
	// 创建一个模拟的HTTP响应
	resp := &http.Response{
		StatusCode: 200,
		Body:       http.NoBody,
	}

	wrappedResp := NewResponse(resp, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = wrappedResp.BodyBytes()
	}
}

// TestThreadSafety 测试客户端的线程安全性
func TestThreadSafety(t *testing.T) {
	// 创建一个客户端实例
	client := NewClient()

	// 在多个 goroutine 中并发使用
	var wg sync.WaitGroup
	const numGoroutines = 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 每次调用 Start() 都会创建独立的请求上下文
			// 不同 goroutine 之间不会相互影响
			resp := client.Start().
				SetURL(fmt.Sprintf("https://httpbin.org/get?id=%d", id)).
				SetHeader("X-Request-ID", fmt.Sprintf("req-%d", id)).
				SetTimeout(5 * time.Second).
				Get(context.Background())

			if resp.Error() != nil {
				t.Logf("Request %d failed: %v", id, resp.Error())
				return
			}

			if !resp.IsSuccess() {
				t.Logf("Request %d failed with status: %d", id, resp.StatusCode())
				return
			}

			t.Logf("Request %d completed successfully with status: %d", id, resp.StatusCode())
		}(i)
	}

	wg.Wait()
	t.Log("All concurrent requests completed")
}

// TestRequestContextIsolation 测试请求上下文的隔离性
func TestRequestContextIsolation(t *testing.T) {
	client := NewClient()

	// 创建两个不同的请求上下文
	ctx1 := client.Start().
		SetURL("https://httpbin.org/get?test=1").
		SetHeader("X-Test", "context1")

	ctx2 := client.Start().
		SetURL("https://httpbin.org/get?test=2").
		SetHeader("X-Test", "context2")

	// 验证两个上下文是独立的
	if ctx1 == ctx2 {
		t.Error("Expected different RequestContext instances")
	}

	// 可以同时配置不同的参数而不相互影响
	ctx1.SetQueryParam("param1", "value1")
	ctx2.SetQueryParam("param2", "value2")

	// 这里我们不实际发送请求，只是验证上下文的独立性
	t.Log("Request contexts are properly isolated")
}
