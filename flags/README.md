# flags

flags 是一个基于 [viper](https://github.com/spf13/viper) 和 [pflag](https://github.com/spf13/pflag) 的高级配置管理包，使应用程序能够简单地处理命令行参数、配置文件以及远程配置中心的配置。

## 特性

- 支持多种数据类型: string, bool, int, float64, duration, slice
- 支持从本地文件或远程配置中心 (ZooKeeper) 读取配置
- 支持 struct 结构体配置绑定
- 支持配置热更新
- 支持必需参数检查
- 简洁的 API 设计

## 基本用法

### API 概览

```go
// 基础类型API
intFlag = flags.Int(key, defaultVal, desc)
strFlag = flags.String(key, defaultVal, desc)
floatFlag = flags.Float64(key, defaultVal, desc)
boolFlag = flags.Bool(key, defaultVal, desc)
durationFlag = flags.Duration(key, defaultVal, desc)
strSliceFlag = flags.StringSlice(key, defaultVal, desc)

// 结构体API
structFlag = flags.Struct(key, defaultVal, desc)

// 必需参数API
requiredStrFlag = flags.StringRequired(key, desc)
requiredIntFlag = flags.IntRequired(key, desc)
requiredBoolFlag = flags.BoolRequired(key, desc)
```

### 获取配置值

```go
// 使用访问器函数获取值
intVal := intFlag()
strVal := strFlag()
floatVal := floatFlag()
boolVal := boolFlag()
duration := durationFlag()
sliceVal := strSliceFlag()

// 结构体配置获取
structVal := new(YourStruct)
err := structFlag(structVal)
```

### 初始化配置

```go
import "github.com/miebyte/goutils/flags"

// 定义配置参数
var (
    // 普通参数
    portFlag = flags.Int("port", 8080, "HTTP服务端口")
    debugFlag = flags.Bool("debug", false, "是否开启调试模式")
    serviceFlag = flags.StringP("serviceName", "s", "", "服务名称")
    
    // 必需参数
    apiKeyFlag = flags.StringRequired("api_key", "API密钥")
    
    // 结构体参数
    configFlag = flags.Struct("config", &MyConfig{}, "应用配置")
)

// 解析配置
flags.Parse()

// 使用配置值
port := portFlag()
debug := debugFlag()
```

## 配置来源

flags 支持从三种不同的来源加载配置：

### 命令行参数（Flags）

通过命令行参数获取配置：

```go
var portFlag = flags.Int("port", 8080, "HTTP服务端口")
```

启动时可以通过命令行参数指定值：
```bash
go run main.go --port 28080
```

### 本地配置文件

flags 默认从以下位置查找配置文件：
- 当前目录 `.`
- `./configs` 目录
- 用户主目录 `$HOME`

默认配置文件名为 `config.json`，可以通过 `-f` 或 `--configFile` 参数指定自定义配置文件：

```bash
./your-app -f custom-config.yaml
```

示例配置文件 `config.json`：
```json
{
    "config": {},
    "redis": {},
    "port": 28080
}
```

对应的代码读取配置：
```go
var (
    configFlag = flags.Struct("config", (*ConfigStruct)(nil), "应用程序配置")
    redisFlag = flags.Struct("redis", (*RedisStruct)(nil), "Redis配置")
    portFlag = flags.Int("port", 8080, "HTTP服务端口")
)
```

### 远程配置中心（ZooKeeper）

使用 `flags.WithRemoteConfig("serviceName")` 可以从 ZooKeeper 获取远程配置。

```go
flags.Parse(
    flags.WithRemoteConfig("your_app_name"),
)
```

ZooKeeper 读取配置需要在根目录下有 `superconf.json` 文件指定环境和 ZK 连接信息：

```json
{
    "env": {
        "name": "dev",
        "zookeeper": {
            "host": "zk-host",
            "port": 2181,
            "auth_data": {
                "scheme": "digest",
                "auth": "user:password"
            }
        },
        "deploy": "dev",
        "group": "group-name"
    }
}
```

#### ZK 配置路径规则

ZK 读取配置文件路径: `/superconf/${env}/${group}/${serviceName}`

默认会读取以下配置并根据 Key 存放到 Viper 中:
- key: `config`. zkPath: `{superconf.ZkPrefix()}/{serviceName}/config`
- key: `mysql`. zkPath: `{superconf.ZkPrefix()}/{serviceName}/mysql`
- key: `redis`. zkPath: `{superconf.ZkPrefix()}/{serviceName}/redis`
- key: `sentry`. zkPath: `{superconf.ZkPrefix()}/{serviceName}/sentry`
- key: `logging`. zkPath: `{superconf.ZkPrefix()}/{serviceName}/logging`
- key: `elasticsearch`. zkPath: `{superconf.ZkPrefix()}/{serviceName}/elasticsearch`
- key: `kafka`. zkPath: `{superconf.ZkPrefix()}/{serviceName}/kafka`
- key: `service`. zkPath: `{superconf.ZkPrefix()}/union/service`
- key: `fluentd`. zkPath: `{superconf.ZkPrefix()}/union/fluentd`

#### 自定义ZK配置

可以使用 `flags.WithZkConfig(path, key, includeService)` 来指定额外的ZK配置路径：
- `{superconf.ZkPrefix()}/{path}` (includeService=false)
- `{superconf.ZkPrefix()}/{serviceName}/{path}` (includeService=true)

```go
flags.Parse(
    flags.WithRemoteConfig("my-service"),
    // 从 /superconf/${env}/${group}/my-service/custom 读取配置，并存储到 key="custom" 下
    flags.WithZkConfig("custom", "custom", true),
)
```

## 结构体配置

```go
// 定义配置结构体
type MyConfig struct {
    Host string `json:"host"`
    Port int    `json:"port"`
    TLS  bool   `json:"tls"`
}

// 实现自定义默认值设置
func (c *MyConfig) SetDefault() {
    if c.Host == "" {
        c.Host = "localhost"
    }
    if c.Port == 0 {
        c.Port = 8443
    }
}

// 实现验证接口
func (c *MyConfig) Validate() error {
    if c.TLS && c.Port == 80 {
        return errors.New("TLS不能使用80端口")
    }
    return nil
}

// 实现配置重载接口
func (c *MyConfig) Reload() {
    // 配置重载时的处理逻辑
    log.Printf("配置已更新: %+v", c)
}

// 在主函数中使用
var myConfig MyConfig
if err := configFlag(&myConfig); err != nil {
    log.Fatalf("加载配置失败: %v", err)
}
```

### 自定义接口

flags支持以下接口来扩展结构体配置的行为：

- `HasDefault` - 设置默认值
- `HasValidator` - 验证配置
- `HasReloader` - 处理配置更新

## 完整示例

```go
package main

import (
    "github.com/miebyte/goutils/logging"
    "github.com/miebyte/goutils/redisutils"
    "github.com/miebyte/goutils/flags"
)

type defaultConf struct {
    Service string
}

var (
    // 配置定义
    redisFlag = flags.Struct("redis", (*redisutils.RedisConfigMap)(nil), "redis配置")
    defaultTestFlag = flags.Struct("defualtConf", &defaultConf{Service: "hoven"}, "默认配置测试")
    portFlag = flags.Int("port", 8080, "服务端口")
)

func main() {
    flags.Parse(
        flags.WithRemoteConfig("work_wechat_msg_archive"),
    )

    defaultConfig := new(defaultConf)
    logging.PanicError(defaultTestFlag(defaultConfig))
    logging.Infof("默认配置: %+v", defaultConfig)

    redisConfig := make(redisutils.RedisConfigMap)
    logging.PanicError(redisFlag(&redisConfig))
    logging.Infof("Redis配置: %+v", logging.Jsonify(redisConfig))

    logging.Infof("端口: %d", portFlag())
}
```

## 配置文件示例

```json
{
    "redis": {
        "task": {
            "host": "localhost",
            "port": 6379,
            "db": 14,
            "password": "",
            "minsize": 100,
            "maxsize": 500,
            "poolsize": 200,
            "timeout": 3
        }
    },
    "port": 28080
}
```