# flags 模块

`flags` 统一读取命令行、JSON 配置和默认值，并支持本地配置监听。它属于 bootstrap/config 边界。

## 基本顺序

Flag 和 Struct Parser 必须在 `Parse` 前声明；每个进程只调用一次 `flags.Parse`。

```go
var (
    portFlag   = flags.Int("port", 8080, "listen port")
    configFlag = flags.Struct("config", (*Config)(nil), "service config")
)

func main() {
    flags.Parse()

    cfg := new(Config)
    logging.PanicError(configFlag(cfg))

    // 初始化 Infrastructure 并启动服务
    _ = portFlag()
}
```

优先级由实现统一处理；不要在业务代码里再次手工合并命令行、配置文件和默认值。

## 配置文件

默认查找 `config.json`，路径依次包含当前目录、`./configs` 和 `$HOME`。也可用 `--configFile/-f` 指定 JSON 文件。

内置参数包括：

- `--serviceName/-s`
- `--serviceTag/-t`
- `--debug`
- `--watchConfig`
- `--configFile/-f`

不要打印完整配置对象，除非已经确认其中没有密码、Token 或 Secret。当前配置监听在 Debug 日志中会输出变更配置，包含敏感字段的服务应谨慎同时启用 Debug 和配置监听。

## `Struct`

`flags.Struct` 仅接受 struct 或 map 类型，并要求 Parser 输出为非 nil 指针。解析顺序为：

1. 读取配置值。
2. 如果实现 `SetDefault()`，应用默认值。
3. 如果实现 `Validate() error`，执行校验。
4. 如果实现 `Reload()`，注册该 key 的重载处理器。

```go
type Config struct {
    Env     string        `json:"env"`
    Timeout time.Duration `json:"timeout"`
}

func (c *Config) SetDefault() {
    if c.Timeout == 0 {
        c.Timeout = 3 * time.Second
    }
}

func (c *Config) Validate() error {
    if c.Env != "dev" && c.Env != "prod" {
        return fmt.Errorf("unsupported env: %s", c.Env)
    }
    return nil
}
```

- `Validate()` 只校验配置，不连接网络、不启动 goroutine、不修改全局状态。
- 必选依赖在组合根创建后校验一次，不要把依赖组装错误延迟到运行期。
- `SetStructParseTagName` 会修改全局解析 Tag，只在整个进程统一采用另一套 Tag 时调用。

## 热更新

使用 `flags.Parse(flags.WithConfigWatch())` 或 `--watchConfig=true` 启用本地文件监听。

实现 `Reload()` 的对象会先在临时对象中重新解析和校验；成功后覆盖原对象，再调用业务 `Reload()`。失败时保留旧值。

重要限制：当前 `Struct` 热更新会原地覆盖对象，不为并发读取提供同步。并发服务不要在多个 goroutine 无保护地读取可变配置；应选择以下之一：

- 不启用热更新。
- 在应用侧将有效配置复制到 `atomic.Pointer` 等只读快照。
- 用锁保护读写，并确保一次业务操作只使用同一份快照。

同一 key 只注册首个重载处理器。不要重复解析同一 key 并期待多个对象同时热更新。

## 多运行模式

只有一个二进制确实有多个独立运行模式时使用 `flags.Func`：

```go
var command = flags.Func(
    "cmd",
    "server",
    "runtime mode",
    flags.WithFunc("server", runServer),
    flags.WithFunc("worker", runWorker),
)
```

单一 HTTP 服务或单一 Worker 直接在 `main.go` 组装，不要为形式完整引入命令分派。
