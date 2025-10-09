// File:		snail.go
// Created by:	Hoven
// Created on:	2025-04-18
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package snail

import "github.com/miebyte/goutils/internal/innerlog"

type slowerObject struct {
	name string
	fn   func() error
}

var (
	objs = make([]*slowerObject, 0)
)

func RegisterObject(name string, fn func() error) {
	objs = append(objs, &slowerObject{
		name: name,
		fn:   fn,
	})
}

func Init() {
	for _, obj := range objs {
		if err := obj.fn(); err != nil {
			innerlog.Logger.PanicError(err, "slow init %s failed", obj.name)
		}

		innerlog.Logger.Debugf("slow init %s success", obj.name)
	}
}
