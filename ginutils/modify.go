package ginutils

import (
	"context"
	"reflect"

	"github.com/miebyte/goutils/structutils"
)

func modifyRequestData(ctx context.Context, reqPtr any, kind reflect.Kind) error {
	switch kind {
	case reflect.Slice, reflect.Array:
		// 切片/数组逐项使用 dive 标签校验
		return structutils.Modifier().Field(ctx, reqPtr, "dive")
	case reflect.Struct:
		// 结构体使用默认的修饰
		return structutils.Modifier().Struct(ctx, reqPtr)
	}

	// 其他类型不进行修饰
	return nil
}
