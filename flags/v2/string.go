// File:		string.go
// Created by:	Hoven
// Created on:	2025-04-01
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package flags

import "github.com/spf13/pflag"

type StringGetter func() string

func (sg StringGetter) Value() string {
	if sg == nil {
		return ""
	}

	return sg()
}

func String(key, defaultVal, usage string) StringGetter {
	pflag.String(key, defaultVal, usage)
	sf.SetDefault(key, defaultVal)
	BindPFlag(key, pflag.Lookup(key))

	return func() string {
		return sf.GetString(key)
	}
}

func StringP(key, shorthand, defaultVal, usage string) StringGetter {
	pflag.StringP(key, shorthand, defaultVal, usage)
	sf.SetDefault(key, defaultVal)
	BindPFlag(key, pflag.Lookup(key))

	return func() string {
		return sf.GetString(key)
	}
}

func StringRequired(key, usage string) StringGetter {
	requiredFlags = append(requiredFlags, key)

	return String(key, "", usage)
}
