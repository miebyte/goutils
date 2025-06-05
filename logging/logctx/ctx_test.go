// File:		with_test.go
// Created by:	Hoven
// Created on:	2025-04-03
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package logctx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFmtKeyValue(t *testing.T) {
	lc := &LogContext{
		Keys:   make([]string, 0),
		Values: make([]string, 0),
	}
	nlc, err := lc.ParseFmtKeyValue("group")
	assert.Nil(t, err)

	t.Logf("newlc: %v", nlc)
	t.Log("==============================")

	lc = &LogContext{
		Keys:   make([]string, 0),
		Values: make([]string, 0),
	}
	nlc, err = lc.ParseFmtKeyValue("key1", "value1")
	assert.Nil(t, err)

	t.Logf("newlc: %v", nlc)
	t.Log("==============================")

	lc = &LogContext{
		Keys:   make([]string, 0),
		Values: make([]string, 0),
	}
	nlc, err = lc.ParseFmtKeyValue("key1", "value1", "key2", "value2")
	assert.Nil(t, err)

	t.Logf("newlc: %v", nlc)
	t.Log("==============================")

	lc = &LogContext{
		Keys:   make([]string, 0),
		Values: make([]string, 0),
	}
	nlc, err = lc.ParseFmtKeyValue("key1", "value1", "key2")
	assert.Nil(t, err)

	t.Logf("newlc: %v", nlc)
	t.Log("==============================")

}
