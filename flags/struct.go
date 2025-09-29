// File:		struct.go
// Created by:	Hoven
// Created on:	2025-04-01
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package flags

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

var (
	ErrNotStruct = errors.New("not struct")

	tag = "json"
)

type StructParser[T any] func(out T) error

func Struct[T any](key string, defaultVal T, usage string) StructParser[T] {
	// 判断 T 的类型，仅支持 struct 和 map
	st := reflect.TypeOf(defaultVal)
	if st.Kind() == reflect.Pointer {
		st = st.Elem()
	}

	if st.Kind() != reflect.Struct && st.Kind() != reflect.Map {
		innerlog.Logger.PanicError(fmt.Errorf("superflags.Struct must be a struct or map, but got %s", st.Kind()))
	}

	sf.SetDefault(key, defaultVal)
	return func(out T) error {
		if reflect.TypeOf(out).Kind() != reflect.Pointer {
			return errors.New("out must be a pointer")
		}

		if err := unmarshalKey(key, out); err != nil {
			return errors.Wrap(err, "UnmarshalKey")
		}

		if err := structCheck(out); err != nil {
			return errors.Wrap(err, "check")
		}

		reloaderCheck(key, out)

		return nil
	}
}

type HasDefault interface {
	SetDefault()
}

type HasValidator interface {
	Validate() error
}

type HasReloader interface {
	Reload()
}

func SetStructParseTagName(t string) {
	tag = t
}

func unmarshalKey(key string, out any) error {
	val := sf.Get(key)
	if val == nil {
		return nil
	}

	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.New("out must be a non-nil pointer")
	}
	rv = rv.Elem()

	// direct assign if types match
	if reflect.TypeOf(val) != nil && reflect.TypeOf(val).AssignableTo(rv.Type()) {
		rv.Set(reflect.ValueOf(val))
		return nil
	}

	// dispatch by kind
	switch rv.Kind() {
	case reflect.Struct:
		m := cast.ToStringMap(val)
		return decodeMapToStruct(rv, m)
	case reflect.Map:
		return setStringAnyMap(rv, val)
	case reflect.Slice:
		return setSlice(rv, val)
	default:
		return setValue(rv, val)
	}
}

func decodeMapToStruct(dst reflect.Value, data map[string]any) error {
	if dst.Kind() == reflect.Pointer {
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		dst = dst.Elem()
	}
	if dst.Kind() != reflect.Struct {
		return errors.New("out must be a struct or pointer to struct")
	}

	typ := dst.Type()
	for i := 0; i < dst.NumField(); i++ {
		field := dst.Field(i)
		fieldType := typ.Field(i)
		if !field.CanSet() {
			continue
		}

		name, ok := fieldEffectiveName(fieldType)
		if !ok {
			continue
		}
		val, exists := data[strings.ToLower(name)]
		if !exists {
			continue
		}

		if err := setValue(field, val); err != nil {
			return err
		}
	}
	return nil
}

func fieldEffectiveName(sf reflect.StructField) (string, bool) {
	tagVal := sf.Tag.Get(tag)
	if tagVal == "-" {
		return "", false
	}
	if tagVal != "" {
		parts := strings.Split(tagVal, ",")
		if parts[0] != "" {
			return parts[0], true
		}
	}
	return sf.Name, true
}

func setValue(field reflect.Value, val any) error {
	typ := field.Type()

	// handle pointers
	if typ.Kind() == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(typ.Elem()))
		}
		return setValue(field.Elem(), val)
	}

	switch typ.Kind() {
	case reflect.Bool:
		field.SetBool(cast.ToBool(val))
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// special case: time.Duration
		if typ.PkgPath() == "time" && typ.Name() == "Duration" {
			field.SetInt(int64(cast.ToDuration(val)))
			return nil
		}
		field.SetInt(int64(cast.ToInt(val)))
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		field.SetUint(uint64(cast.ToUint(val)))
		return nil
	case reflect.Float32, reflect.Float64:
		field.SetFloat(cast.ToFloat64(val))
		return nil
	case reflect.String:
		field.SetString(cast.ToString(val))
		return nil
	case reflect.Interface:
		if val == nil {
			field.Set(reflect.Zero(field.Type()))
		} else {
			field.Set(reflect.ValueOf(val))
		}
		return nil
	case reflect.Struct:
		m := cast.ToStringMap(val)
		return decodeMapToStruct(field, m)
	case reflect.Slice:
		return setSlice(field, val)
	case reflect.Map:
		return setStringAnyMap(field, val)
	default:
		return nil
	}
}

func setSlice(field reflect.Value, val any) error {
	elem := field.Type().Elem()
	arr := cast.ToSlice(val)
	out := reflect.MakeSlice(field.Type(), 0, len(arr))
	for _, it := range arr {
		newElem := reflect.New(elem).Elem()
		if err := setValue(newElem, it); err != nil {
			return err
		}
		out = reflect.Append(out, newElem)
	}
	field.Set(out)
	return nil
}

func setStringAnyMap(field reflect.Value, val any) error {
	if field.Type().Key().Kind() != reflect.String {
		return nil
	}
	m := cast.ToStringMap(val)
	out := reflect.MakeMapWithSize(field.Type(), len(m))
	for k, v := range m {
		newVal := reflect.New(field.Type().Elem()).Elem()
		if err := setValue(newVal, v); err != nil {
			return err
		}
		out.SetMapIndex(reflect.ValueOf(k), newVal)
	}
	field.Set(out)
	return nil
}

func structCheck(out any) error {
	if d, ok := out.(HasDefault); ok {
		d.SetDefault()
	}
	if v, ok := out.(HasValidator); ok {
		return v.Validate()
	}

	return nil
}

func reloaderCheck(key string, out any) {
	d, ok := out.(HasReloader)
	if ok {
		RegisterReloadFunc(key, &tmpConfigReloader{d, key})
	}
}

type tmpConfigReloader struct {
	out HasReloader
	key string
}

func (t *tmpConfigReloader) reUnmarshalReloader() error {
	rValueOf := reflect.ValueOf(t.out)
	if rValueOf.Kind() == reflect.Pointer {
		rValueOf = rValueOf.Elem()
	}

	tempValue := reflect.New(rValueOf.Type())
	if err := unmarshalKey(t.key, tempValue.Interface()); err != nil {
		return err
	}

	rValueOf.Set(tempValue.Elem())

	return nil
}

func (t *tmpConfigReloader) Reload() {
	if err := t.reUnmarshalReloader(); err != nil {
		innerlog.Logger.Errorf("reunmarshal %s error: %v", t.key, err)
		return
	}

	if err := structCheck(t.out); err != nil {
		innerlog.Logger.Errorf("%s check error: %v", t.key, err)
		return
	}

	t.out.Reload()
}
