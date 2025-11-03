// File:		client.go
// Created by:	Hoven
// Created on:	2025-01-XX
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package emailutils

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
)

// EmailClient 邮件客户端
type EmailClient struct {
	config *EmailConfig
}

// NewEmailClient 创建邮件客户端
func NewEmailClient(config *EmailConfig) *EmailClient {
	return &EmailClient{
		config: config,
	}
}

// Send 发送邮件
func (c *EmailClient) Send(to, subject, body string) error {
	from := c.config.From
	if c.config.FromName != "" {
		from = fmt.Sprintf("%s <%s>", c.config.FromName, c.config.From)
	}

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	auth := smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.Host)

	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", to, from, subject, body))

	var err error
	if c.config.Port == 465 {
		// SSL连接
		err = c.sendEmailTLS(addr, auth, from, []string{to}, msg)
	} else {
		// 普通连接或STARTTLS
		err = smtp.SendMail(addr, auth, c.config.From, []string{to}, msg)
	}

	return err
}

// sendEmailTLS 使用TLS发送邮件（用于465端口）
func (c *EmailClient) sendEmailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: c.config.Host,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, c.config.Host)
	if err != nil {
		return err
	}
	defer client.Close()

	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}

	if err = client.Mail(from); err != nil {
		return err
	}

	for _, addr := range to {
		if err = client.Rcpt(addr); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}

	_, err = w.Write(msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}
