package reflectx

import "reflect"

// ResolveBaseKind 返回类型最终的基础 Kind，忽略指针层级。
func ResolveBaseKind(t reflect.Type) reflect.Kind {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind()
}

// IndirectValue 递归剥离可解引用的指针，直到命中非指针或 nil。
func IndirectValue(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Pointer && !v.IsNil() {
		v = v.Elem()
	}
	return v
}
