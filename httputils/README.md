# HTTP Utils - Go HTTP 客户端工具包

这个包提供了一个功能强大、易于使用的 HTTP 客户端，支持链式调用、中间件和丰富的配置选项。

## 特性

- 🔗 **链式调用**: 流畅的 API 设计，支持方法链式调用
- 🔌 **中间件支持**: 可插拔的中间件系统，支持请求/响应处理
- ⚡ **高性能**: 基于标准库 `net/http`，性能优异
- 🛡️ **类型安全**: 完全的 Go 类型安全
- 🧵 **线程安全**: 每次 `Start()` 创建独立的请求上下文，支持并发使用
- 📊 **内置功能**: 提供日志记录、超时控制、错误处理等功能
- 🎯 **灵活配置**: 支持超时、请求头、查询参数、JSON 请求体等配置

## 快速开始

### 基本用法

可直接通过 httputils.Client 使用 net/http 接口。

兼容 net/http：`Client` 内嵌 `*http.Client`，可直接调用 `client.Do(req)`、`client.Get(url)` 等标准方法。

注意：`client.Do(req)` 不会经过通过 `Use()` 注册的中间件链；如需使用中间件，请通过 `client.Start()` 链式调用，或将已有请求传入 `SetRequest(req)` 再执行。

```go
ctx := context.Background()

client := httputils.NewClient()

req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://httpbin.org/get", nil)
if err != nil {
    panic(err)
}
req.Header.Set("User-Agent", "MyApp/1.0")

resp, err := client.Do(req)
if err != nil {
    panic(err)
}
defer resp.Body.Close()
```

使用 httputils.Client 的链式调用方法

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/miebyte/goutils/httputils"
)

func main() {
    // 创建客户端
    client := httputils.NewClient()
    
    // 发送 GET 请求
    resp := client.Start().
        SetURL("https://httpbin.org/get").
        SetQueryParam("key", "value").
        SetHeader("User-Agent", "MyApp/1.0").
        SetTimeout(10 * time.Second).
        Get(context.Background())
        
    // 检查错误
    if resp.Error() != nil {
        fmt.Printf("Error: %v\n", resp.Error())
        return
    }
    
    // 检查响应状态
    if !resp.IsSuccess() {
        fmt.Printf("Request failed with status: %d\n", resp.StatusCode())
        return
    }
    
    // 获取响应体
    body, err := resp.BodyString()
    if err != nil {
        fmt.Printf("Failed to read body: %v\n", err)
        return
    }
    
    fmt.Printf("Response: %s\n", body)
}
```

### 使用中间件

```go
func main() {
    client := httputils.NewClient()
    
    // 添加中间件
    client.Use(
        httputils.HandlerDuration(), // 持续时间记录中间件
    )
    
    // 发送请求
    resp := client.Start().
        SetURL("https://httpbin.org/get").
        Get(context.Background())
        
    // 处理响应...
}
```

### POST JSON 请求

```go
func main() {
    client := httputils.NewClient()
    
    // 使用 map 作为 JSON 数据
    data := map[string]interface{}{
        "name":  "John Doe",
        "email": "john@example.com",
    }
    
    resp := client.Start().
        SetURL("https://httpbin.org/post").
        SetJSONBody(data). // 自动设置 Content-Type: application/json
        SetHeader("Authorization", "Bearer token123").
        Post(context.Background())
        
    if resp.Error() != nil {
        fmt.Printf("Error: %v\n", resp.Error())
        return
    }
    
    // 解析JSON响应
    var result map[string]interface{}
    if err := resp.BodyJson(&result); err != nil {
        fmt.Printf("Failed to parse JSON: %v\n", err)
        return
    }
    
    fmt.Printf("Response: %+v\n", result)
}
```

### 使用 JsonBody 构建器

```go
func main() {
    client := httputils.NewClient()
    
    // 使用 JsonBody 构建器
    jsonBody := httputils.NewJsonBody().
        Add("name", "test").
        Add("value", 123).
        Add("active", true)
    
    resp := client.Start().
        SetURL("https://httpbin.org/post").
        SetJSONBody(jsonBody).
        Post(context.Background())
        
    // 处理响应...
}
```

### 响应处理

客户端返回封装的 `Response` 类型，提供便捷的响应处理方法：

```go
func main() {
    client := httputils.NewClient()
    resp := client.Start().
        SetURL("https://httpbin.org/json").
        Get(context.Background())
    
    // 检查错误
    if resp.Error() != nil {
        fmt.Printf("Request failed: %v\n", resp.Error())
        return
    }
    
    // 检查状态码
    fmt.Printf("Status Code: %d\n", resp.StatusCode())
    fmt.Printf("Is Success: %t\n", resp.IsSuccess())
    
    // 获取响应体 - 字符串形式
    bodyStr, err := resp.BodyString()
    if err != nil {
        fmt.Printf("Failed to get body string: %v\n", err)
        return
    }
    fmt.Printf("Body String: %s\n", bodyStr)
    
    // 获取响应体 - 字节数组形式
    bodyBytes, err := resp.BodyBytes()
    if err != nil {
        fmt.Printf("Failed to get body bytes: %v\n", err)
        return
    }
    fmt.Printf("Body Length: %d bytes\n", len(bodyBytes))
    
    // 解析JSON响应
    var jsonData map[string]interface{}
    if err := resp.BodyJson(&jsonData); err != nil {
        fmt.Printf("Failed to parse JSON: %v\n", err)
        return
    }
    fmt.Printf("JSON Data: %+v\n", jsonData)
}
```

### 线程安全使用

客户端设计为线程安全，可以在多个 goroutine 中并发使用：

```go
func main() {
    // 创建一个客户端实例
    client := httputils.NewClient()
    client.Use(httputils.HandlerDuration())
    
    // 在多个 goroutine 中并发使用
    var wg sync.WaitGroup
    
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            
            // 每次调用 Start() 都会创建独立的请求上下文
            // 不同 goroutine 之间不会相互影响
            resp := client.Start().
                SetURL(fmt.Sprintf("https://httpbin.org/get?id=%d", id)).
                SetHeader("X-Request-ID", fmt.Sprintf("req-%d", id)).
                Get(context.Background())
                
            if resp.Error() != nil {
                fmt.Printf("Request %d failed: %v\n", id, resp.Error())
                return
            }
            
            fmt.Printf("Request %d completed with status: %d\n", id, resp.StatusCode())
        }(i)
    }
    
    wg.Wait()
}
```

## API 参考

### Client 方法

#### 客户端创建和配置

- `NewClient(opts ...clientOptionFn) *Client` - 创建新的客户端实例
- `WithTimeout(timeout time.Duration) clientOptionFn` - 设置客户端超时选项
- `WithTransport(tp http.RoundTripper) clientOptionFn` - 设置自定义传输层
- `Use(handlers ...HandleFunc) *Client` - 添加中间件处理器
- `Start() *RequestContext` - 开始构建新请求，返回独立的请求上下文（线程安全）

#### 与 net/http.Client 的兼容性

- `Client` 内嵌 `*http.Client`，因此可直接调用标准方法：`client.Do(req)`、`client.Get(url)`、`client.Post(url, ct, body)`、`client.Head(url)`、`client.PostForm(url, data)` 等。
- 默认 `Transport` 已包装透传与追踪：`NewPassThroughRoundTripper` → `trace.NewTracingRoundTripper`。通过 `WithTransport` 仅替换底层传输，包装仍会生效。
- 注意：通过 `client.Do(req)` 发起的请求不会经过用 `Use()` 注册的中间件链；需要中间件时请使用 `client.Start()` 的链式调用或将已有 `*http.Request` 传入 `SetRequest(req)` 再执行。

#### 请求上下文方法

以下方法在 `RequestContext` 上调用，通过 `client.Start()` 获得：

- `SetURL(url string) *RequestContext` - 设置请求 URL
- `SetHeader(key, value string) *RequestContext` - 设置单个请求头
- `SetHeaders(headers map[string]string) *RequestContext` - 设置多个请求头
- `SetQueryParam(key, value string) *RequestContext` - 设置单个查询参数
- `SetQueryParams(params Params) *RequestContext` - 设置多个查询参数
- `SetBody(body io.Reader) *RequestContext` - 设置请求体
- `SetJSONBody(data any) *RequestContext` - 设置 JSON 请求体
- `SetTimeout(timeout time.Duration) *RequestContext` - 设置超时时间
- `SetRequest(req *http.Request) *RequestContext` - 设置自定义请求对象
- `AddError(err error)` - 追加错误（用于中间件/链式构建阶段）
- `GetError() error` - 获取当前上下文中的错误
- `Next()` - 执行下一个中间件
- `Abort()` - 终止中间件链执行

#### HTTP 方法

所有 HTTP 方法都返回 `*Response` 类型：

- `Get(ctx context.Context) *Response` - 发送 GET 请求
- `Post(ctx context.Context) *Response` - 发送 POST 请求
- `Put(ctx context.Context) *Response` - 发送 PUT 请求
- `Delete(ctx context.Context) *Response` - 发送 DELETE 请求
- `Patch(ctx context.Context) *Response` - 发送 PATCH 请求
- `Head(ctx context.Context) *Response` - 发送 HEAD 请求
- `Options(ctx context.Context) *Response` - 发送 OPTIONS 请求

### Response 方法

`Response` 类型封装了 HTTP 响应，提供便捷的处理方法：

#### 错误和状态检查

- `Error() error` - 返回请求过程中的错误（如果有）
- `StatusCode() int` - 返回 HTTP 状态码
- `IsSuccess() bool` - 检查响应是否成功（状态码 200-299）

#### 响应体处理

- `BodyBytes() ([]byte, error)` - 获取响应体的字节数组
- `BodyString() (string, error)` - 获取响应体的字符串形式
- `BodyJson(v any) error` - 将响应体解析为 JSON 对象

#### 原始响应访问

- `*http.Response` - 嵌入的原始 HTTP 响应，可直接访问所有标准字段和方法

```go
// 访问原始响应
resp := client.Start().SetURL("https://example.com").Get(ctx)
headers := resp.Header
contentType := resp.Header.Get("Content-Type")
statusText := resp.Status
```

### 工具类型

#### Header 工具

```go
// 创建 Header
header := httputils.NewHeader()
header.Add("Content-Type", "application/json")
header.BasicAuth("username", "password")

// 预定义 Header
jsonHeader := httputils.DefaultJsonHeader()
formHeader := httputils.DefaultFormUrlEncodedHeader()
multipartHeader := httputils.DefaultFormHeader()
```

#### Params 工具

```go
// 创建查询参数
params := httputils.NewParams()
params.Add("page", "1")
params.Add("limit", "10")

// 获取参数值
value := params.Get("page")
```

#### JsonBody 构建器

```go
// 创建 JSON 请求体
jsonBody := httputils.NewJsonBody().
    Add("name", "John").
    Add("age", 30).
    Add("active", true)

// 编码为字节数组
bytes := jsonBody.Encode()
```

#### StatusCodeResponseWriter

用于在服务端侧捕获响应状态码（不影响写入流程）：

```go
rw := httputils.NewStatusCodeResponseWriter(w)
handler(rw, r)
statusCode := rw.StatusCode
```

### 内置中间件

#### 持续时间记录中间件

```go
// 记录请求持续时间并输出日志
durationHandler := httputils.HandlerDuration()
client.Use(durationHandler)
```

### 自定义中间件

实现 `HandleFunc` 类型来创建自定义中间件：

```go
func AuthMiddleware(token string) httputils.HandleFunc {
    return func(c *httputils.RequestContext) {
        // 在请求前添加认证头
        c.SetHeader("Authorization", "Bearer "+token)
        
        // 调用下一个处理器
        c.Next()
        
        // 在响应后进行处理
        if c.GetError() == nil && c.response != nil && c.response.StatusCode == 401 {
            // 处理认证失败
            fmt.Println("Authentication failed")
        }
    }
}

// 使用自定义中间件
client.Use(AuthMiddleware("your-token"))
```

### 便捷函数

包还提供了一些便捷函数，用于快速发送请求：

```go
// 快速 GET 请求
resp, err := httputils.Get(ctx, "https://example.com")

// 快速 POST 请求
resp, err := httputils.Post(ctx, "https://example.com", body)

// 快速 PUT 请求
resp, err := httputils.Put(ctx, "https://example.com", body)

// 快速 DELETE 请求
resp, err := httputils.Delete(ctx, "https://example.com")
```

## 中间件执行顺序

中间件按照添加的顺序执行，形成一个处理链：

```go
client.Use(middleware1, middleware2, middleware3)

// 执行顺序：
// middleware1 -> middleware2 -> middleware3 -> HTTP请求 -> middleware3 -> middleware2 -> middleware1
```

## 性能考虑

- 每次调用 `Start()` 都会创建新的请求上下文，适合复用同一个客户端实例
- 中间件链在每次请求时构建，开销很小
- 底层使用标准库 `net/http.Client`，支持连接池和 Keep-Alive
- 内置 OpenTracing 支持，用于分布式追踪
- `BodyBytes()` 会读取并关闭响应体；`BodyString()`/`BodyJson()` 同样会消费响应体，如需多次读取请自行缓存

## 错误处理

包使用 `github.com/hashicorp/go-multierror` 来处理多个错误：

```go
resp := client.Start().
    SetURL("invalid-url").
    SetJSONBody(invalidData).
    Get(ctx)

if resp.Error() != nil {
    // 可能包含多个错误（URL 解析错误、JSON 编码错误等）
    fmt.Printf("Errors: %v\n", resp.Error())
}
```

## 示例

更多使用示例请参考 `client_test.go` 文件。