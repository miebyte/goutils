# websocketutils 模块

`websocketutils` 提供基于 `http.Handler` 的 WebSocket Server、命名空间、房间、连接事件和 JSON Frame。它位于 Interface/Infrastructure 边界；事件 Handler 应调用 Application Service。

## 服务装配

```go
ws := websocketutils.NewServer(
    websocketutils.WithCheckOrigin(checkOrigin),
    websocketutils.WithAllowRequestFunc(authenticateRequest),
    websocketutils.WithHeartbeat(20*time.Second, 60*time.Second),
)

chat := ws.Of("chat")
chat.On("send_message", func(c *websocketutils.Context) {
    var req SendMessageRequest
    if err := c.Scan(&req); err != nil {
        return
    }
    _ = app.Send(c.Context(), req)
})

srv := cores.NewCores(cores.WithHttpHandler("/ws", ws))
```

上例的公开地址 `/ws/chat` 经 `cores.WithHttpHandler` 去掉 `/ws` 后，WebSocket Server 收到 `/chat`，因此进入 `chat` 命名空间。`WithNamespacePrefix` 是针对 Handler 实际收到的 URL Path；组合使用时根据最终路径测试，不要重复配置已被 cores 剥离的前缀，否则连接会落入 `default`。

## Frame 与事件

客户端和服务端使用 JSON：

```json
{"event":"send_message","data":{"text":"hello"}}
```

- `Context.Scan` 将 `data` 解码到目标对象。
- `Conn.Emit` 只发给当前连接。
- `Namespace.Emit` 广播命名空间。
- `Namespace.To(room).Emit` 广播一个或多个房间。
- 每个连接自动加入以自身连接 ID 命名的房间，可用于定向推送。
- 连接级 Handler 优先于 Namespace Handler。

## 安全与生命周期

- 默认 `CheckOrigin` 返回 true。生产环境必须通过 `WithCheckOrigin` 限制允许来源，不能依赖默认值。
- `WithAllowRequestFunc` 在 Upgrade 前执行，可校验鉴权并返回替换后的 Request；拒绝会返回 HTTP 403。
- 将鉴权身份放入 Request Context，再通过事件 `Context.Context()` 传给 Application。
- Event Handler panic 会被捕获并记录，但业务错误仍应显式处理和反馈。连接和命名空间 Handler 在读循环中顺序执行，长耗时 Handler 会阻塞后续消息与心跳处理；保持处理快速，必要时在明确顺序、背压和 Context 语义后再异步化。
- 发送队列满时返回 `ErrBufferFull`，连接关闭返回 `ErrConnClosed`；调用方要定义丢弃、断开或重试策略，禁止无限重试。
- Heartbeat 使用内置 `ping`/`pong` 事件名；业务事件不要复用。
- `Rooms()` 和 `Members()` 的返回顺序不应作为契约。
