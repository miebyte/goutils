package structutils

import (
	"github.com/go-playground/locales/zh"
	"github.com/go-playground/validator/v10"
	"github.com/miebyte/goutils/logging"

	ut "github.com/go-playground/universal-translator"
	zh_translations "github.com/go-playground/validator/v10/translations/zh"
)

var (
	validate *validator.Validate
	trans    ut.Translator
)

func init() {
	validate = validator.New()
	chinese := zh.New()
	trans, _ = ut.New(chinese, chinese).GetTranslator("zh")
	err := zh_translations.RegisterDefaultTranslations(validate, trans)
	if err != nil {
		logging.Errorf("init validator translation failed: %v", err)
	}
}

func RegisterValidation(tag string, fn validator.Func, callValidationEvenIfNull ...bool) {
	validate.RegisterValidation(tag, fn, callValidationEvenIfNull...)
}

func RegisterValidateAlias(alias, tag string) {
	validate.RegisterAlias(alias, tag)
}

func RegisterValidateCustomTypeFunc(fn validator.CustomTypeFunc, types ...any) {
	validate.RegisterCustomTypeFunc(fn, types...)
}

func RegisterStructValidation(fn validator.StructLevelFunc, types ...any) {
	validate.RegisterStructValidation(fn, types...)
}

func RegisterStructValidationMapRules(rules map[string]string, types ...any) {
	validate.RegisterStructValidationMapRules(rules, types...)
}

// RegisterValidationTranslation 注册自定义翻译
func RegisterValidationTranslation(tag string, registerFn validator.RegisterTranslationsFunc, translationFn validator.TranslationFunc) error {
	return validate.RegisterTranslation(tag, trans, registerFn, translationFn)
}

// RegisterValidationSimpleTranslation 注册简单模板翻译
// 支持传入自定义参数函数，用于构建翻译参数
// tag: 翻译标签
// translation: 翻译模板
// override: 是否覆盖已有翻译
// argsFn: 自定义参数函数
// 示例：
//
//	RegisterValidationSimpleTranslation("required", "不能为空", true, func(fe validator.FieldError) []string {
//		return []string{fe.Field()}
//	})
func RegisterValidationSimpleTranslation(tag, translation string, override bool, argsFn ...func(validator.FieldError) []string) error {
	builder := func(fe validator.FieldError) []string {
		return []string{fe.Field(), fe.Param()}
	}
	if len(argsFn) > 0 && argsFn[0] != nil {
		builder = argsFn[0]
	}

	registerFn := func(t ut.Translator) error {
		return t.Add(tag, translation, override)
	}

	translationFn := func(t ut.Translator, fe validator.FieldError) string {
		args := builder(fe)
		tpl, err := t.T(tag, args...)
		if err != nil {
			logging.Warnf("validator translation fallback tag=%s err=%v field=%s", tag, err, fe.Field())
			return fe.(error).Error()
		}
		return tpl
	}

	return RegisterValidationTranslation(tag, registerFn, translationFn)
}

func Validator() *validator.Validate {
	return validate
}

func TranslateErr(e validator.FieldError) string {
	return e.Translate(trans)
}
