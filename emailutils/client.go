// File:		client.go
// Created by:	Hoven
// Created on:	2025-01-XX
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package emailutils

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"mime"
	"net/mail"
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
	// 构建发件人地址
	fromAddr := &mail.Address{
		Name:    c.config.FromName,
		Address: c.config.From,
	}

	// 构建收件人地址
	toAddr, err := mail.ParseAddress(to)
	if err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}

	// 构建邮件消息
	msg, err := c.buildMessage(fromAddr, toAddr, subject, body)
	if err != nil {
		return fmt.Errorf("failed to build message: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	auth := smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.Host)

	if c.config.Port == 465 {
		// SSL连接
		return c.sendEmailTLS(addr, auth, c.config.From, []string{to}, msg)
	}

	// 普通连接或STARTTLS
	return smtp.SendMail(addr, auth, c.config.From, []string{to}, msg)
}

// buildMessage 构建邮件消息
func (c *EmailClient) buildMessage(from, to *mail.Address, subject, body string) ([]byte, error) {
	var buf bytes.Buffer

	// 编码主题（支持非ASCII字符）
	encodedSubject := mime.QEncoding.Encode("UTF-8", subject)

	// 写入邮件头
	buf.WriteString(fmt.Sprintf("From: %s\r\n", from.String()))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to.String()))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", encodedSubject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	buf.WriteString("\r\n")

	// 写入邮件正文
	buf.WriteString(body)

	return buf.Bytes(), nil
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
