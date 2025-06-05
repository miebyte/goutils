// File:		error.go
// Created by:	Hoven
// Created on:	2025-04-25
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package dialer

import "github.com/pkg/errors"

var (
	ErrServiceNotExists = errors.New("service not exists")
)
