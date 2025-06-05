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

	"github.com/go-viper/mapstructure/v2"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

var (
	ErrNotStruct = errors.New("not struct")

	keyStructMap = make(map[string]any)

	tag = "json"
)

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

func withViperTagName() viper.DecoderConfigOption {
	return func(dc *mapstructure.DecoderConfig) {
		dc.TagName = tag
	}
}

func Struct[T any](key string, defaultVal T, usage string) func(out T) error {
	v.SetDefault(key, defaultVal)
	return func(out T) error {
		if reflect.TypeOf(out).Kind() != reflect.Ptr {
			return errors.New("out must be a pointer")
		}

		if err := v.UnmarshalKey(key, out, withViperTagName()); err != nil {
			return errors.Wrap(err, "UnmarshalKey")
		}

		if err := structCheck(out); err != nil {
			return errors.Wrap(err, "check")
		}

		keyStructMap[key] = out

		return nil
	}
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

func structConfReload() {
	for key, out := range keyStructMap {
		if err := v.UnmarshalKey(key, out, withViperTagName()); err != nil {
			fmt.Printf("reunmarshal %s error: %v\n", key, err)
			continue
		}

		if err := structCheck(out); err != nil {
			fmt.Printf("%s check error: %v\n", key, err)
			continue
		}

		if r, ok := out.(HasReloader); ok {
			r.Reload()
		}
	}
}
