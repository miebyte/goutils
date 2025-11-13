# Prometheus Go 监控工具

这是一个完整的 Prometheus 监控工具包，提供了服务注册、指标收集、HTTP 服务器等功能，适用于 Go 微服务监控。

## 功能特性

- **HTTP 服务器**: 提供 `/metrics` 端点供 Prometheus 抓取
- **资源监控**: 定时监控服务器资源（TCP 连接数等）
- **指标收集**: 支持多种类型的指标（Counter、Gauge、Histogram）
- **多模块监控**: 支持 DNS、API、限流、脚本执行、服务器资源等监控

## 监控模块

### 支持的监控类型

| 模块 | 类型 | 指标名称 | 说明 |
|------|------|----------|------|
| DNS 监控 | Gauge | `dns_duration_gauge` | DNS 解析耗时 |
| API 监控 | Counter | `api_counter` | API 访问次数 |
| API 监控 | Histogram | `api_histogram` | API 访问耗时分布 |
| 限流监控 | Counter | `rate_limit_counter` | 限流触发次数 |
| 脚本监控 | Counter | `script_execute_times` | 脚本执行次数 |
| 服务器监控 | Gauge | `tcp_connection_gauge` | TCP 连接数 |
| 自定义监控 | Counter | `custom_counter` | 自定义计数器 |

### 限流类型

- `RateLimitIP`: IP 限流
- `RateLimitUser`: 用户限流  
- `RateLimitAPI`: API 限流
- `RateLimitCustom`: 自定义限流

## 环境变量配置

| 变量名 | 说明 | 默认值 | 必需 |
|--------|------|--------|------|
| `POD_IP` | Pod 的 IP 地址 | - | 是 |

## 使用方法

### 基本使用

#### 自动发现注册地址

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"
    
    "gitlab.weike.fm/mircoservice/go-utils/prometheus"
)

func main() {
    ctx := context.Background()
    port := 9090
    
    // 注册 Prometheus 服务（自动发现注册地址）
    if err := prometheus.SetupPrometheus(ctx, port); err != nil {
        log.Printf("注册 Prometheus 服务失败: %v", err)
    }
    
    // 启动 HTTP 服务器
    http.Handle("/metrics", prometheus.PrometheusHttpHandler())
    
    // 启动资源监控（可选）
    go func() {
        monitorFunc := prometheus.RegularMonitorServerResources(30 * time.Second)
        if err := monitorFunc(ctx); err != nil {
            log.Printf("资源监控失败: %v", err)
        }
    }()
    
    log.Printf("Prometheus 服务启动在端口 %d", port)
    log.Fatal(http.ListenAndServe(":9090", nil))
}
```

#### 指定注册地址

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"
    
    "gitlab.weike.fm/mircoservice/go-utils/prometheus"
)

func main() {
    ctx := context.Background()
    port := 9090
    registerUrl := "http://consul:8500" // 指定注册服务地址
    
    // 注册 Prometheus 服务（指定注册地址）
    if err := prometheus.SetupPrometheusWithRegisterUrl(ctx, registerUrl, port); err != nil {
        log.Printf("注册 Prometheus 服务失败: %v", err)
    }
    
    // 启动 HTTP 服务器
    http.Handle("/metrics", prometheus.PrometheusHttpHandler())
    
    log.Printf("Prometheus 服务启动在端口 %d", port)
    log.Fatal(http.ListenAndServe(":9090", nil))
}
```

### 发送监控指标

```go
// 发送自定义计数器
prometheus.SendCustomCounter("user_module")

// 发送 API 监控
prometheus.SendAPICounter()
prometheus.SendAPIHistogram("/api/users", 150.5, 200)

// 发送 DNS 监控
prometheus.SendDNSGauge("example.com", 25.3)

// 发送限流监控
prometheus.SendRateLimitCounter(prometheus.RateLimitIP, "192.168.1.1")

// 发送脚本执行监控
prometheus.SendScriptExecuteCounter("backup.sh")

// 发送 TCP 连接数监控（定时执行）
prometheus.SendCurrentTCPConnectionGauge()
```

## API 参考

### 服务注册函数

#### SetupPrometheus
```go
func SetupPrometheus(ctx context.Context, port int) error
```
自动发现注册地址并注册 Prometheus 服务。

#### SetupPrometheusWithRegisterUrl
```go
func SetupPrometheusWithRegisterUrl(ctx context.Context, registerUrl string, port int) error
```
使用指定的注册地址注册 Prometheus 服务。

#### RegisterPrometheusService
```go
func RegisterPrometheusService(ctx context.Context, port int) error
```
自动发现注册地址并注册 Prometheus 服务（内部函数）。

#### RegisterPrometheusServiceWithRegisterUrl
```go
func RegisterPrometheusServiceWithRegisterUrl(ctx context.Context, registerUrl string, port int) error
```
使用指定的注册地址注册 Prometheus 服务（内部函数）。

### 服务注册信息

注册的服务信息包含：
- **ID**: Pod 名称
- **Name**: 服务名称
- **Tags**: 标签数组（app, monitor, 服务名, 命名空间, Pod名）
- **Address**: Pod IP 地址
- **Port**: 服务端口
- **Checks**: 健康检查配置（HTTP 检查，30秒间隔）

## 指标标签

所有指标都包含以下公共标签：

- `server_name`: 服务名称
- `pod_namespace`: Pod 命名空间
- `pod_name`: Pod 名称
- `module`: 监控模块

## 注意事项

1. 确保设置了正确的环境变量，特别是 `POD_IP`
2. TCP 连接数监控仅在 Linux 系统上有效（读取 `/proc/1/net/snmp`）
3. 指标会自动注册到 Prometheus 客户端库的默认注册表中
4. 服务注册需要网络连接到注册服务（如 Consul）
5. 注册地址格式应为：`http://host:port` 或 `https://host:port`
