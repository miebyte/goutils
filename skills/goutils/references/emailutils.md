# emailutils 模块

`emailutils` 提供简单 SMTP HTML 邮件发送。它属于 Infrastructure Adapter；Application 应依赖业务语义接口。

```go
type NotificationSender interface {
    SendOrderCreated(ctx context.Context, to string, orderID string) error
}
```

Infrastructure 实现可持有 `*emailutils.EmailClient`：

```go
client := emailutils.NewEmailClient(&emailutils.EmailConfig{
    Host:     cfg.Host,
    Port:     cfg.Port,
    Username: cfg.Username,
    Password: cfg.Password,
    From:     cfg.From,
    FromName: cfg.FromName,
})

err := client.Send(to, subject, htmlBody)
```

## 行为与限制

- 端口 `465` 使用直接 TLS；其他端口调用 `smtp.SendMail`。
- Body 固定按 `text/html; charset=UTF-8` 发送。
- 当前 `Send` 不接收 Context，也没有显式超时参数。不要在数据库事务内同步发送；需要超时、取消、重试或批量发送时在 Adapter 外层封装或改用更合适的实现。
- 在组合根校验 Host、Port、Username、From 等必选配置。
- 密码不得写入日志、错误响应或测试快照。
- 收件地址是外部输入时先做产品级校验；发送失败由 Application 决定重试、补偿或记录状态。
