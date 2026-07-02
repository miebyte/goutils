// File:		utils.go
// Created by:	Hoven
// Created on:	2025-04-03
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package logging

import (
	"encoding/json"
	"reflect"
	"runtime"
	"strings"
	"time"
)

func GetStructName(i any) string {
	tPtr := reflect.TypeOf(i)

	if tPtr.Kind() == reflect.Ptr {
		tPtr = tPtr.Elem()
	}

	return tPtr.Name()
}

func GetFuncName(fn any) string {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		return "nonfunc"
	}
	pc := v.Pointer()
	f := runtime.FuncForPC(pc)
	if f == nil {
		return "unknown"
	}
	return f.Name()
}

func GetShortFuncName(fn any) string {
	full := GetFuncName(fn)
	if full == "" {
		return full
	}

	if i := strings.LastIndex(full, "."); i >= 0 && i+1 < len(full) {
		return full[i+1:]
	}

	return full
}

// TimeFuncDuration returns the duration consumed by function.
// It has specified usage like:
//
//	    f := TimeFuncDuration()
//		   DoSomething()
//		   duration := f()
func TimeFuncDuration() func() time.Duration {
	start := time.Now()
	return func() time.Duration {
		return time.Since(start)
	}
}

func TimeDurationDefer(prefix ...string) func() {
	ps := "operation"
	if len(prefix) != 0 {
		ps = strings.Join(prefix, ", ")
	}
	start := time.Now()

	return func() {
		Infof("%v elapsed time: %v", ps, time.Since(start))
	}
}

func Jsonify(v any) string {
	d, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		Errorf("jsonify error: %v", err)
		panic(err)
	}
	return string(d)
}

func JsonifyNoIndent(v interface{}) string {
	d, err := json.Marshal(v)
	PanicError(err)
	return string(d)
}
