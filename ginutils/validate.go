package ginutils

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/structutils"
	"github.com/miebyte/goutils/utils/reflectx"
)

// validateRequestData 校验请求参数
func validateRequestData(ctx context.Context, reqPtr any, reqKind reflect.Kind) error {
	if reqPtr == nil {
		return nil
	}

	if reqKind == reflect.Slice || reqKind == reflect.Array {
		// 在切片场景下，统一解引用到实际容器值
		reqVal := reflect.ValueOf(reqPtr)
		if reqVal.Kind() == reflect.Pointer {
			reqVal = reqVal.Elem()
		}
		reqVal = reflectx.IndirectValue(reqVal)
		switch {
		// 无有效值时跳过校验
		case !reqVal.IsValid():
			return nil
		// 顶层指针仍为 nil，直接通过
		case reqVal.Kind() == reflect.Pointer && reqVal.IsNil():
			return nil
		// 切片/数组逐项使用 dive 标签校验
		case reqVal.Kind() == reflect.Slice || reqVal.Kind() == reflect.Array:
			return structutils.Validator().VarCtx(ctx, reqVal.Interface(), "dive")
		// 退化为结构体校验
		default:
			return structutils.Validator().Struct(reqPtr)
		}
	}
	return structutils.Validator().Struct(reqPtr)
}

func handleValidateError(c *gin.Context, err error) {
	if err != nil {
		errStrs := []string{}
		verrs := new(validator.ValidationErrors)
		if errors.As(err, verrs) {
			for _, e := range *verrs {
				errStrs = append(errStrs, structutils.TranslateErr(e))
			}
		} else {
			errStrs = append(errStrs, err.Error())
		}
		errStr := strings.Join(errStrs, ";")
		logging.Errorc(c.Request.Context(), "validator req data failed. error: %v", errStr)
		ReturnError(c, http.StatusBadRequest, errStr)
		return
	}
}
