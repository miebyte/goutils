// File:		masking.go
// Created by:	Hoven
// Created on:	2025-05-13
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.
package masking

import (
	"regexp"
)

var (
	// 是否启用敏感数据脱敏功能
	enableMasking bool = false

	// 敏感数据值匹配规则列表
	valuePatterns []ValuePattern

	// 默认掩码字符
	defaultMask string = "*****"
)

type ValuePattern struct {
	// 正则表达式模式，直接匹配值内容
	Pattern *regexp.Regexp

	// 掩码替换函数
	Replacer func(string) string

	// 快速检查函数，用于提前过滤不需要正则匹配的内容
	QuickCheck func(string) bool
}

func EnableMasking(enable bool) {
	enableMasking = enable
}

func IsMaskingEnabled() bool {
	return enableMasking
}

func SetDefaultMask(mask string) {
	defaultMask = mask
}

func ClearMaskingRules() {
	valuePatterns = []ValuePattern{}
}

func AddValuePattern(pattern string, replacer func(string) string) *PatternBuilder {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return &PatternBuilder{err: err}
	}

	valuePattern := ValuePattern{
		Pattern:  re,
		Replacer: replacer,
		QuickCheck: func(s string) bool {
			return true
		},
	}

	index := len(valuePatterns)
	valuePatterns = append(valuePatterns, valuePattern)
	return &PatternBuilder{index: index}
}

type PatternBuilder struct {
	index int
	err   error
}

// AddQuickCheck 添加快速检查函数
func (b *PatternBuilder) AddQuickCheck(checker func(string) bool) *PatternBuilder {
	if b.err != nil {
		return b
	}

	if checker != nil && b.index >= 0 && b.index < len(valuePatterns) {
		valuePatterns[b.index].QuickCheck = checker
	}

	return b
}

func (b *PatternBuilder) Error() error {
	return b.err
}

// AddPhonePattern 添加手机号脱敏规则
func AddPhonePattern() error {
	phonePattern := `\b1[3-9]\d{9}\b`

	builder := AddValuePattern(phonePattern, func(s string) string {
		if len(s) != 11 {
			return s
		}
		return s[:3] + defaultMask + s[len(s)-4:]
	})

	return builder.Error()
}

// AddIDCardPattern 添加身份证号脱敏规则
func AddIDCardPattern() error {
	idCardPattern := `\b[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`

	builder := AddValuePattern(idCardPattern, func(s string) string {
		if len(s) != 18 {
			return s
		}
		return s[:6] + "********" + s[len(s)-4:]
	})

	return builder.Error()
}

func MaskMessage(message string) string {
	if !enableMasking || message == "" {
		return message
	}

	result := message

	for _, pattern := range valuePatterns {
		if pattern.QuickCheck != nil && !pattern.QuickCheck(result) {
			continue
		}

		if pattern.Replacer != nil {
			result = pattern.Pattern.ReplaceAllStringFunc(result, pattern.Replacer)
		} else {
			result = pattern.Pattern.ReplaceAllString(result, defaultMask)
		}
	}

	return result
}
