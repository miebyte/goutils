// File:		viper.go
// Created by:	Hoven
// Created on:	2025-04-01
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package flags

import (
	"fmt"

	"github.com/spf13/pflag"
)

func BindPFlag(key string, flag *pflag.Flag) {
	if err := v.BindPFlag(key, flag); err != nil {
		fmt.Printf("BindPFlag key: %v, err: %v\n", key, err)
	}
}
