// File:		binding.go
// Created by:	Hoven
// Created on:	2025-06-05
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package ginutils

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type bindStrategy interface {
	Need(c *gin.Context) bool
	Bind(c *gin.Context, obj any) error
}

type headerBind struct{}

func (b *headerBind) Need(c *gin.Context) bool {
	return len(c.Request.Header) > 0
}

func (b *headerBind) Bind(c *gin.Context, obj any) error {
	return c.ShouldBindHeader(obj)
}

type urlBind struct{}

func (b *urlBind) Need(c *gin.Context) bool {
	return len(c.Params) > 0
}

func (b *urlBind) Bind(c *gin.Context, obj any) error {
	return c.ShouldBindUri(obj)
}

type queryBind struct{}

func (b *queryBind) Need(c *gin.Context) bool {
	return len(c.Request.URL.Query()) > 0
}

func (b *queryBind) Bind(c *gin.Context, obj any) error {
	return c.ShouldBindQuery(obj)
}

type bodyBind struct{}

func (b *bodyBind) Need(c *gin.Context) bool {
	return c.Request.ContentLength > 0
}

func (b *bodyBind) Bind(c *gin.Context, obj any) error {
	binder := binding.Default(c.Request.Method, c.ContentType())
	return c.ShouldBindWith(obj, binder)
}
