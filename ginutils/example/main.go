package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/ginutils"
)

func main() {
	ginutils.NewServerHandler(
		ginutils.WithPrefix("/api/v1"),
		ginutils.WithGroupHandlers(
			ginutils.WithPrefix("/outer"),
			// 认证管理组
			ginutils.WithGroupHandlers(
				ginutils.WithPrefix("/auth"),
				ginutils.WithHandler(http.MethodGet, "/", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{"message": "Hello, World!"})
				}),
			),
			// 用户管理组
			ginutils.WithGroupHandlers(
				ginutils.WithPrefix("/user"),
				// ginutils.WithMiddleware(authMiddleware),
				ginutils.WithHandler(http.MethodGet, "/", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{"message": "Hello, World!"})
				}),
				// routerHandler is implement by Router interface
				ginutils.WithRouterHandler(),
			),
			// 账单管理组
			ginutils.WithGroupHandlers(
				ginutils.WithPrefix("/bill"),
				// ginutils.WithMiddleware(authMiddleware),
				ginutils.WithHandler(http.MethodGet, "/", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{"message": "Hello, World!"})
				}),
				// routerHandler is implement by Router interface
				ginutils.WithRouterHandler(),
			),
		),
		ginutils.WithGroupHandlers(
			ginutils.WithPrefix("/inner"),
			ginutils.WithHandler(http.MethodGet, "/user/static", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "Hello, World!"})
			}),
		),
	)
}
