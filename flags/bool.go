// File:		bool.go
// Created by:	Hoven
// Created on:	2025-04-01
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package flags

import "github.com/spf13/pflag"

type BoolGetter func() bool

func (bg BoolGetter) Value() bool {
	if bg == nil {
		return false
	}

	return bg()
}

func Bool(key string, defaultVal bool, usage string) BoolGetter {
	pflag.Bool(key, defaultVal, usage)
	sf.SetDefault(key, defaultVal)
	BindPFlag(key, pflag.Lookup(key))

	return func() bool {
		return sf.GetBool(key)
	}
}

func BoolRequired(key, usage string) BoolGetter {
	requiredFlags = append(requiredFlags, key)
	return Bool(key, false, usage)
}
