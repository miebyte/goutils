// File:		config.go
// Created by:	Hoven
// Created on:	2025-01-XX
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package emailutils

// EmailConfig SMTP邮件配置
type EmailConfig struct {
	Host     string `json:"host"`      // SMTP服务器地址
	Port     int    `json:"port"`      // SMTP端口，如587、465、25
	Username string `json:"username"`  // SMTP用户名
	Password string `json:"password"`  // SMTP密码
	From     string `json:"from"`      // 发件人邮箱地址
	FromName string `json:"from_name"` // 发件人名称
}
