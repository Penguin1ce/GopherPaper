// Package router 装配 gin 引擎与路由，处理函数来自 handler 包。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/handler"
	chathandler "GopherPaper/internal/handler/chat"
	paperhandler "GopherPaper/internal/handler/paper"
	"GopherPaper/internal/handler/user"
	wshandler "GopherPaper/internal/handler/ws"
	"GopherPaper/internal/middleware"
	"GopherPaper/internal/response"
	"GopherPaper/internal/web"
	"GopherPaper/internal/zlog"
)

// Init 构建 gin 引擎。mode 为运行模式，依赖已由各包 Init 初始化。
func Init(mode string) *gin.Engine {
	gin.SetMode(mode)
	r := gin.New()
	// 访问日志与 panic 恢复都写到 zlog 的输出目标，和应用日志同去向（文件或 stdout）。
	r.Use(gin.LoggerWithWriter(zlog.Writer()), gin.RecoveryWithWriter(zlog.Writer()))

	r.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", web.IndexHTML())
	})
	// 精读页是独立入口,新标签页打开,前端按 ?id 渲染 PDF。
	r.GET("/reader", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", web.ReaderHTML())
	})
	r.StaticFS("/static", http.FS(web.Static()))

	// 健康检查，无需鉴权。
	r.GET("/healthz", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	{
		// 公开接口：签发 token。
		api.POST("/auth/token", handler.Token)

		// 公开接口：用户注册登录与邮箱验证码。
		api.POST("/user/send-code", user.SendCode) // 下发邮箱验证码
		api.POST("/user/register", user.Register)  // 校验验证码并注册
		api.POST("/user/login", user.Login)        // 登录签发 JWT

		// WebSocket 订阅解析进度，握手鉴权走 query token。
		api.GET("/ws", wshandler.Subscribe)

		// 取召回引用的图片，img 标签带不了头，鉴权走 query token。
		api.GET("/papers/:id/figures/:name", paperhandler.Figure)

		// 取原始 PDF，pdf.js 带不了头，鉴权走 query token。
		api.GET("/papers/:id/file", paperhandler.File)

		// 受保护接口：JWT 校验后注入租户身份。
		authed := api.Group("")
		authed.Use(middleware.JWTAuth())
		{
			// 论文上传与管理。
			authed.POST("/papers", paperhandler.Upload)            // 上传 PDF，触发异步解析
			authed.GET("/papers", paperhandler.List)               // 列出我的论文
			authed.GET("/papers/search", paperhandler.Search)      // 历史文献检索
			authed.GET("/papers/:id", paperhandler.Detail)         // 论文详情与结构化元信息
			authed.GET("/papers/:id/status", paperhandler.Status)  // 解析状态兜底查询
			authed.POST("/papers/:id/report", paperhandler.Report)       // 生成研读报告
			authed.POST("/papers/:id/translate", paperhandler.Translate) // 精读页逐段翻译

			// 会话与多轮论文问答。
			authed.POST("/sessions", chathandler.CreateSession)            // 新建会话，可绑定论文
			authed.GET("/sessions", chathandler.ListSessions)              // 列出我的会话
			authed.DELETE("/sessions/:id", chathandler.DeleteSession)      // 删除会话
			authed.GET("/sessions/:id/messages", chathandler.ListMessages) // 拉取历史消息
			authed.POST("/sessions/:id/messages", chathandler.SendMessage) // 发消息，Host 路由专家
		}
	}
	return r
}
