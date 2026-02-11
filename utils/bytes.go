// File:		bytes.go
// Created by:	Hoven
// Created on:	2025-04-25
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"time"
)

func Md5(src any) []byte {
	h := md5.New()

	switch val := src.(type) {
	case []byte:
		h.Write(val)
	case string:
		h.Write([]byte(val))
	default:
		h.Write(fmt.Append(nil, src))
	}

	bs := h.Sum(nil)
	dst := make([]byte, hex.EncodedLen(len(bs)))
	hex.Encode(dst, bs)
	return dst
}

func ShortMd5(src any) []byte {
	return Md5(src)[8:24]
}

func AppendAny(dst []byte, v any) []byte {
	if v == nil {
		return append(dst, "<nil>"...)
	}

	switch val := v.(type) {
	case []byte:
		dst = append(dst, val...)
	case string:
		dst = strconv.AppendQuote(dst, val)
	case int:
		dst = strconv.AppendInt(dst, int64(val), 10)
	case int8:
		dst = strconv.AppendInt(dst, int64(val), 10)
	case int16:
		dst = strconv.AppendInt(dst, int64(val), 10)
	case int32:
		dst = strconv.AppendInt(dst, int64(val), 10)
	case int64:
		dst = strconv.AppendInt(dst, val, 10)
	case uint:
		dst = strconv.AppendUint(dst, uint64(val), 10)
	case uint8:
		dst = strconv.AppendUint(dst, uint64(val), 10)
	case uint16:
		dst = strconv.AppendUint(dst, uint64(val), 10)
	case uint32:
		dst = strconv.AppendUint(dst, uint64(val), 10)
	case uint64:
		dst = strconv.AppendUint(dst, val, 10)
	case float32:
		dst = strconv.AppendFloat(dst, float64(val), 'f', -1, 32)
	case float64:
		dst = strconv.AppendFloat(dst, val, 'f', -1, 64)
	case bool:
		dst = strconv.AppendBool(dst, val)
	case time.Time:
		dst = val.AppendFormat(dst, time.RFC3339)
	case time.Duration:
		dst = strconv.AppendInt(dst, int64(val), 10)
	case error:
		dst = append(dst, val.Error()...)
	case fmt.Stringer:
		dst = append(dst, val.String()...)
	default:
		dst = appendComplexType(dst, v)
	}
	return dst
}

func appendComplexType(dst []byte, v any) []byte {
	// 复杂类型统一走稳定编码，避免 map 顺序不确定。
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return append(dst, "<nil>"...)
	}
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return append(dst, "<nil>"...)
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Map:
		type kv struct {
			key     reflect.Value
			sortKey string
		}
		keys := rv.MapKeys()
		items := make([]kv, 0, len(keys))
		for _, k := range keys {
			ki := k.Interface()
			items = append(items, kv{
				key:     k,
				sortKey: fmt.Sprintf("%T:%v", ki, ki),
			})
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].sortKey < items[j].sortKey
		})

		dst = append(dst, '{')
		for i, it := range items {
			kb := AppendAny(nil, it.key.Interface())
			vb := AppendAny(nil, rv.MapIndex(it.key).Interface())
			dst = append(dst, kb...)
			dst = append(dst, ':')
			dst = append(dst, vb...)
			if i < len(items)-1 {
				dst = append(dst, ',')
			}
		}
		dst = append(dst, '}')
		return dst
	case reflect.Slice, reflect.Array:
		dst = append(dst, '[')
		for i := 0; i < rv.Len(); i++ {
			elem := rv.Index(i).Interface()
			eb := AppendAny(nil, elem)
			dst = append(dst, eb...)
			if i < rv.Len()-1 {
				dst = append(dst, ',')
			}
		}
		dst = append(dst, ']')
		return dst
	}

	return append(dst, fmt.Sprint(v)...)
}
