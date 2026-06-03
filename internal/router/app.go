// Package router 装配 gin 引擎与路由，处理函数来自 handler 包。
package router

import (
	"github.com/gin-gonic/gin"

	"GopherCPP/internal/handler"
	"GopherCPP/internal/middleware"
)

// Init 构建 gin 引擎。mode 为运行模式，依赖已由各包 Init 初始化。
func Init(mode string) *gin.Engine {
	gin.SetMode(mode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// 健康检查，无需鉴权。
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	{
		// 公开接口：签发 token。
		api.POST("/auth/token", handler.Token)

		// 受保护接口：JWT 校验后注入租户身份。
		authed := api.Group("")
		authed.Use(middleware.JWTAuth())
		{
			authed.POST("/chat", handler.Chat)   // 聊天答疑，经意图识别
			authed.POST("/exam", handler.Exam)   // 出题按钮，结构化参数
			authed.POST("/grade", handler.Grade) // 批改按钮，结构化参数
			// 后续扩展：私有知识库上传等
		}
	}
	return r
}
