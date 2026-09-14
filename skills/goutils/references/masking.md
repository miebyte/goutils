# masking 模块

`masking` 对字符串中的手机号、身份证或自定义正则值做脱敏，适合日志和诊断输出边界。

```go
logging.PanicError(masking.AddPhonePattern())
logging.PanicError(masking.AddIDCardPattern())
masking.EnableMasking(true)

safe := masking.MaskMessage(rawMessage)
```

自定义规则：

```go
err := masking.AddValuePattern(`token=[^&\s]+`, nil).
    AddQuickCheck(func(s string) bool { return strings.Contains(s, "token=") }).
    Error()
```

## 约束

- 默认关闭；启用前先注册所需规则。
- `SetDefaultMask` 影响使用默认替换的规则，也会影响之后手机号规则的替换行为。
- 注册表和开关是包级可变状态，没有并发保护；在进程启动阶段一次配置，不在请求处理中动态清空、注册或切换。
- 脱敏是最后防线，不等于允许记录 Secret。密码、私钥、访问 Token 等应从源头禁止进入日志。
- 正则必须有测试，覆盖边界、误匹配和 Unicode/多行输入；必要时使用 `AddQuickCheck` 降低大文本开销。
