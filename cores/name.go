// File:		func.go
// Created by:	Hoven
// Created on:	2025-04-02
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package cores

import (
	"reflect"
	"runtime"
)

func GetStructName(i any) string {
	tPtr := reflect.TypeOf(i)
	if tPtr == nil {
		return ""
	}

	if tPtr.Kind() == reflect.Pointer {
		tPtr = tPtr.Elem()
	}

	return tPtr.Name()
}

func GetFuncName(i any) string {
	if i == nil {
		return ""
	}
	val := reflect.ValueOf(i)
	if val.Kind() != reflect.Func {
		return ""
	}
	fn := runtime.FuncForPC(val.Pointer())
	if fn == nil {
		return ""
	}
	return fn.Name()
}
