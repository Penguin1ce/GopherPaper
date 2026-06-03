// Package router 装配 gin 引擎与路由。
package router

import (
	"github.com/gin-gonic/gin"

	"GopherCPP/internal/auth"
	"GopherCPP/internal/controller"
	"GopherCPP/internal/middleware"
	"GopherCPP/internal/service"
)

// New 构建 gin 引擎。mode 为运行模式，mgr 做 JWT 签发与校验。
func New(mode string, mgr *auth.Manager, assistant *service.Assistant) *gin.Engine {
	gin.SetMode(mode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// 健康检查，无需鉴权。
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	authCtl := controller.NewAuthController(mgr)
	chatCtl := controller.NewChatController(assistant)

	api := r.Group("/api/v1")
	{
		// 公开接口：签发 token。
		api.POST("/auth/token", authCtl.Token)

		// 受保护接口：JWT 校验后注入租户身份。
		authed := api.Group("")
		authed.Use(middleware.JWTAuth(mgr))
		{
			authed.POST("/chat", chatCtl.Chat)
			// 后续扩展：试卷、作业提交、私有知识库上传等
		}
	}
	return r
}
