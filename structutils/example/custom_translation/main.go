package main

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"

	"github.com/miebyte/goutils/structutils"
)

type SignupReq struct {
	Username string `validate:"required"`
	Password string `validate:"required,min=6"`
	Status   int    `validate:"required,status_must_1"`
}

func registerValidationAndTranslation() {
	// 注册自定义校验
	structutils.RegisterValidation("status_must_1", func(fl validator.FieldLevel) bool {
		return fl.Field().Int() == 1
	})

	// 给自定义校验 tag 注册翻译
	_ = structutils.RegisterValidationSimpleTranslation(
		"status_must_1",
		"{0}必须为1，当前提交值：{1}",
		true,
		func(fe validator.FieldError) []string {
			return []string{fe.Field(), fmt.Sprint(fe.Value())}
		},
	)

	_ = structutils.RegisterValidationSimpleTranslation(
		"required",
		"{0}不能为空，请重新输入",
		true,
		func(fe validator.FieldError) []string {
			return []string{fe.Field()}
		},
	)
}

func main() {
	registerValidationAndTranslation()

	req := SignupReq{
		Username: "admin",
		Password: "",
		Status:   2,
	}

	if err := structutils.Validator().Struct(req); err != nil {
		var verrs validator.ValidationErrors
		if errors.As(err, &verrs) {
			for _, fe := range verrs {
				fmt.Println(structutils.TranslateErr(fe))
			}
			return
		}
		fmt.Println("unexpected error:", err)
		return
	}

	fmt.Println("校验通过")
}
