## structutils

面向结构体的“修饰 + 校验”工具集合，基于 `go-playground/mold` 与 `go-playground/validator` 进行二次封装：
- **修饰器 Modifier**：使用 mold 对结构体字段做规范化处理（如裁剪、格式化、清洗等）。
- **校验器 Validator**：使用 validator 进行字段与结构体级别校验，内置中文翻译。


### 功能概览

- **Validator（单例）**：
  - `Validator() *validator.Validate`
  - `TranslateErr(e validator.FieldError) string`
  - 支持注册：`RegisterValidation`、`RegisterValidateAlias`、`RegisterValidateCustomTypeFunc`、`RegisterStructValidation`、`RegisterStructValidationMapRules`
  - 支持翻译：`RegisterValidationTranslation`、`RegisterValidationSimpleTranslation`
  - 初始化即加载中文翻译（`zh`），错误信息可直接翻译为中文

- **Modifier（单例）**：
  - `Modifier() *mold.Transformer`
  - 支持注册：`RegisterModifier`、`RegisterModifierAlias`
  - 可在处理入参时统一做字段规范化

### 快速使用

#### 1) 入参修饰（mold）

注册自定义修饰函数，并在请求到达时对结构体入参执行修饰。

```go
// 注册一个示例修饰器（建议在 init 中注册）
structutils.RegisterModifier("nickname_fix", func(ctx context.Context, fl mold.FieldLevel) error {
    f := fl.Field()
    if f.Kind() == reflect.String {
        f.SetString(strings.TrimSpace(f.String()))
    }
    return nil
})

// 在处理请求时执行修饰
if err := structutils.Modifier().Struct(ctx, reqPtr); err != nil {
    // 处理修饰错误
}
```

#### 2) 入参校验（validator）

对结构体字段加上校验 tag，统一用全局 `Validator()` 执行校验，并将错误翻译为中文。

```go
type CreateUserReq struct {
    Name  string `validate:"required,min=2"`
    Email string `validate:"required,email"`
}

if err := structutils.Validator().Struct(reqPtr); err != nil {
    var verrs validator.ValidationErrors
    if errors.As(err, &verrs) {
        msgs := make([]string, 0, len(verrs))
        for _, e := range verrs {
            msgs = append(msgs, structutils.TranslateErr(e))
        }
        // 将 msgs 合并返回
    } else {
        // 处理其它错误
    }
}
```

### 扩展与注册

- 修饰器相关：
  - `RegisterModifier(tag string, fn mold.Func)` 注册自定义修饰函数
  - `RegisterModifierAlias(alias, tag string)` 为已有修饰创建别名

- 校验器相关：
  - `RegisterValidation(tag string, fn validator.Func, callValidationEvenIfNull ...bool)` 自定义字段级校验
  - `RegisterValidateAlias(alias, tag string)` 为已有校验创建别名
  - `RegisterValidateCustomTypeFunc(fn validator.CustomTypeFunc, types ...any)` 自定义类型的反射解析
  - `RegisterStructValidation(fn validator.StructLevelFunc, types ...any)` 结构体级校验
  - `RegisterStructValidationMapRules(rules map[string]string, types ...any)` 基于 map 的结构体级规则
  - `RegisterValidationTranslation(tag string, registerFn validator.RegisterTranslationsFunc, translationFn validator.TranslationFunc)` 注册自定义翻译函数
  - `RegisterValidationSimpleTranslation(tag, translation string, override bool, argsFn ...func(validator.FieldError) []string)` 通过模板快速配置翻译

建议在应用启动阶段（如 `init()`）完成上述注册，避免并发时动态修改造成不确定性。

### 自定义校验翻译

使用 `RegisterValidationSimpleTranslation` 可以覆盖内置提示或为自定义标签增加翻译，`override` 置为 `true` 即可覆盖默认翻译。完整范例见 `example/custom_translation/main.go`：

```go
func init() {
    structutils.RegisterValidation("username_unique", someValidator)

    structutils.RegisterValidationSimpleTranslation(
        "required",
        "{0}不能为空，请重新输入",
        true,
        func(fe validator.FieldError) []string {
            return []string{fe.Field()}
        },
    )
}
```

### 与 Web 框架配合

典型用法是在绑定请求参数后，先执行 `Modifier().Struct(ctx, reqPtr)` 做“清洗/规范化”，再执行 `Validator().Struct(reqPtr)` 做“校验”。项目中的 `ginlibs` 与 `fiberlibs` 已按此顺序集成。

### 依赖

- `github.com/go-playground/mold/v4`
- `github.com/go-playground/validator/v10`
- `github.com/go-playground/locales/zh` 与 `github.com/go-playground/validator/v10/translations/zh`（用于中文翻译）


