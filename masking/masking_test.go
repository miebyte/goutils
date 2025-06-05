// File:		masking_test.go
// Created by:	Hoven
// Created on:	2025-05-13
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package masking

import (
	"strings"
	"testing"
)

func TestMasking(t *testing.T) {
	// 启用脱敏功能
	EnableMasking(true)
	defer EnableMasking(false) // 测试结束后关闭

	// 设置默认掩码
	defaultMask := "*****"
	SetDefaultMask(defaultMask)

	// 测试启用/禁用脱敏功能
	t.Run("EnableDisableMasking", func(t *testing.T) {
		ClearMaskingRules()
		// 添加一个简单的数字匹配规则用于测试
		err := AddValuePattern(`\d+`, func(s string) string {
			return defaultMask
		}).Error()
		if err != nil {
			t.Fatalf("添加值匹配规则失败: %v", err)
		}

		// 启用状态下应该脱敏
		EnableMasking(true)
		result := MaskMessage("12345")
		if result != defaultMask {
			t.Errorf("启用脱敏失败，期望值: %s，实际值: %s", defaultMask, result)
		}

		// 禁用状态下不应该脱敏
		EnableMasking(false)
		result = MaskMessage("12345")
		if result != "12345" {
			t.Errorf("禁用脱敏失败，期望值: %s，实际值: %s", "12345", result)
		}
	})

	// 测试手机号脱敏
	t.Run("PhoneValuePattern", func(t *testing.T) {
		ClearMaskingRules()
		EnableMasking(true)
		err := AddPhonePattern()
		if err != nil {
			t.Fatalf("添加手机号脱敏规则失败: %v", err)
		}

		// 直接对值进行脱敏
		phone := "13800138000"
		expected := "138" + defaultMask + "8000"
		result := MaskMessage(phone)
		if result != expected {
			t.Errorf("手机号脱敏失败，期望值: %s，实际值: %s", expected, result)
		}

		// 测试非手机号不会被脱敏
		notPhone := "123456"
		result = MaskMessage(notPhone)
		if result != notPhone {
			t.Errorf("不应该脱敏，期望值: %s，实际值: %s", notPhone, result)
		}
	})

	// 测试身份证号脱敏
	t.Run("IDCardValuePattern", func(t *testing.T) {
		ClearMaskingRules()
		EnableMasking(true)
		err := AddIDCardPattern()
		if err != nil {
			t.Fatalf("添加身份证号脱敏规则失败: %v", err)
		}

		idCard := "110101199001011234"
		expected := "110101" + "********" + "1234"
		result := MaskMessage(idCard)
		if result != expected {
			t.Errorf("身份证号脱敏失败，期望值: %s，实际值: %s", expected, result)
		}
	})

	// 测试自定义值模式脱敏
	t.Run("CustomValuePattern", func(t *testing.T) {
		ClearMaskingRules()
		EnableMasking(true)
		err := AddValuePattern(`\b\d{16}\b`, func(s string) string {
			return s[:6] + "****" + s[len(s)-4:]
		}).Error()
		if err != nil {
			t.Fatalf("添加自定义值脱敏规则失败: %v", err)
		}

		cardNo := "6225760079930812"
		expected := "622576" + "****" + "0812"
		result := MaskMessage(cardNo)
		if result != expected {
			t.Errorf("自定义值脱敏失败，期望值: %s，实际值: %s", expected, result)
		}
	})

	// 测试直接消息脱敏
	t.Run("DirectMessageMasking", func(t *testing.T) {
		ClearMaskingRules()
		EnableMasking(true)
		AddPhonePattern()
		AddIDCardPattern()

		message := "用户手机号: 13800138000, 身份证: 110101199001011234"
		expected := "用户手机号: 138" + defaultMask + "8000, 身份证: 110101" + "********" + "1234"
		result := MaskMessage(message)
		if result != expected {
			t.Errorf("直接消息脱敏失败，\n期望值: %s，\n实际值: %s", expected, result)
		}
	})

	// 测试多种模式同时存在
	t.Run("MultiplePatterns", func(t *testing.T) {
		ClearMaskingRules()
		EnableMasking(true)
		AddPhonePattern()
		AddIDCardPattern()

		// 银行卡号模式
		AddValuePattern(`\b\d{16,19}\b`, func(s string) string {
			if len(s) < 8 {
				return s
			}
			return s[:6] + "******" + s[len(s)-4:]
		})

		message := "手机:13800138000 身份证:110101199001011234 银行卡:6225760079930812"
		expected := "手机:138" + defaultMask + "8000 身份证:110101" + "********" + "1234 银行卡:622576******0812"
		result := MaskMessage(message)
		if result != expected {
			t.Errorf("多模式脱敏失败，\n期望值: %s，\n实际值: %s", expected, result)
		}
	})

	// 测试快速检查优化
	t.Run("QuickCheck", func(t *testing.T) {
		ClearMaskingRules()
		EnableMasking(true)

		// 使用链式调用添加快速检查
		AddValuePattern(`\d+`, func(s string) string {
			return defaultMask
		}).AddQuickCheck(func(s string) bool {
			// 只对包含"数字"字样的内容进行检查
			return strings.Contains(s, "数字")
		})

		// 应该脱敏：包含"数字"字样且有数字
		message1 := "这是数字123456"
		result1 := MaskMessage(message1)
		expected1 := "这是数字" + defaultMask
		if result1 != expected1 {
			t.Errorf("带QuickCheck的脱敏失败，\n期望值: %s，\n实际值: %s", expected1, result1)
		}

		// 不应该脱敏：不包含"数字"字样，即使有数字
		message2 := "这是987654"
		result2 := MaskMessage(message2)
		if result2 != message2 {
			t.Errorf("QuickCheck应该跳过匹配，\n期望值: %s，\n实际值: %s", message2, result2)
		}
	})
}
