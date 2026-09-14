# structutils 模块

`structutils` 封装 mold 数据修饰器和 go-playground validator，并提供中文错误翻译。它适合 API 输入和配置边界，不替代 Domain 不变量。

## 常用流程

```go
if err := structutils.Modifier().Struct(ctx, input); err != nil {
    return err
}
if err := structutils.Validator().StructCtx(ctx, input); err != nil {
    return err
}
```

在 `ginutils.RequestHandler`/`RequestResponseHandler` 中，上述清洗和校验已经自动执行，不要重复调用。

## 扩展

- `RegisterModifier`、`RegisterModifierAlias`：注册清洗规则。
- `RegisterValidation`、`RegisterValidateAlias`：注册字段校验。
- `RegisterStructValidation`：注册结构级校验。
- `RegisterValidationSimpleTranslation`：为标签增加或覆盖中文翻译。
- `TranslateErr`：翻译单个 `validator.FieldError`。

注册通常在进程初始化阶段一次完成。不要在每次请求中动态注册规则。

## 边界

- modifier 用于 trim、规范化等无争议转换，不承担会决定业务结果的规则。
- validator 用于协议和结构约束；聚合状态转换、额度、库存等业务不变量仍在 Domain。
- 自定义翻译不得泄漏被校验的密码、Token 等原值。
