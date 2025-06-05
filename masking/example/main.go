package main

import (
	"fmt"
	"strings"

	"github.com/miebyte/goutils/masking"
)

func main() {
	masking.EnableMasking(true)
	masking.AddPhonePattern()
	masking.AddIDCardPattern()

	// 添加银行卡号脱敏规则
	masking.AddValuePattern(`\b\d{16,19}\b`, func(s string) string {
		if len(s) < 10 {
			return s
		}
		return s[:6] + "******" + s[len(s)-4:]
	})

	// 添加Email脱敏规则
	masking.AddValuePattern(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`, func(s string) string {
		atIndex := -1
		for i, c := range s {
			if c == '@' {
				atIndex = i
				break
			}
		}

		if atIndex <= 2 {
			return "****" + s[atIndex:]
		}

		return s[:2] + "****" + s[atIndex:]
	})

	rawMessage := "用户资料: 姓名=张三, 手机=13800138000, 身份证=110101199001011234, 银行卡=6225760079930812, 邮箱=user@example.com"
	maskedMessage := masking.MaskMessage(rawMessage)

	fmt.Println("原始内容: " + rawMessage)
	fmt.Println("脱敏后: " + maskedMessage)

	fmt.Println("\n=== 脱敏性能优化 ===")

	// 创建一个只对包含金额的文本进行处理的规则
	masking.ClearMaskingRules()
	masking.AddValuePattern(`\d+\.\d{2}`, func(s string) string {
		return "***.** 元"
	}).AddQuickCheck(func(s string) bool {
		// 只有包含"金额"或"元"字样的文本才会进行正则匹配
		return strings.Contains(s, "金额") || strings.Contains(s, "元")
	})

	text1 := "订单金额为: 1234.56元" // 应该脱敏
	text2 := "请输入数字: 1234.56"  // 不应该脱敏，没有包含"金额"或"元"

	fmt.Printf("原始文本1: %s\n", text1)
	fmt.Printf("脱敏后: %s\n", masking.MaskMessage(text1))
	fmt.Printf("原始文本2: %s\n", text2)
	fmt.Printf("脱敏后: %s\n", masking.MaskMessage(text2))
}
