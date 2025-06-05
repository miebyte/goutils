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

	if tPtr.Kind() == reflect.Ptr {
		tPtr = tPtr.Elem()
	}

	return tPtr.Name()
}

func GetFuncName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
