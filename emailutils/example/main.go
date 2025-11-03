package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/miebyte/goutils/emailutils"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/utils"
)

func main() {
	// 从环境变量读取配置，如果没有设置则使用默认值
	port, err := strconv.Atoi(utils.GetEnvByDefualt("SMTP_PORT", "465"))
	if err != nil {
		log.Fatalf("Invalid SMTP_PORT: %v", err)
	}

	emailConfig := &emailutils.EmailConfig{
		Host:     utils.GetEnvByDefualt("SMTP_HOST", "smtp.example.com"),
		Port:     port,
		Username: utils.GetEnvByDefualt("SMTP_USERNAME", "your_email@example.com"),
		Password: utils.GetEnvByDefualt("SMTP_PASSWORD", "your_password"),
		From:     utils.GetEnvByDefualt("SMTP_FROM", "your_email@example.com"),
		FromName: utils.GetEnvByDefualt("SMTP_FROM_NAME", "系统通知"),
	}

	logging.Infof("emailConfig: %v", logging.Jsonify(emailConfig))

	// 创建邮件客户端
	emailClient := emailutils.NewEmailClient(emailConfig)

	// 发送邮件
	to := "269085434@qq.com"
	subject := "测试邮件"
	body := `
		<!DOCTYPE html>
		<html>
		<head>
			<meta charset="UTF-8">
			<style>
				body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
				.container { max-width: 600px; margin: 0 auto; padding: 20px; }
			</style>
		</head>
		<body>
			<div class="container">
				<h2>测试邮件</h2>
				<p>这是一封测试邮件。</p>
			</div>
		</body>
		</html>
	`

	err = emailClient.Send(to, subject, body)
	if err != nil {
		log.Fatalf("Failed to send email: %v", err)
	}
	fmt.Printf("邮件已发送到 %s\n", to)
}
