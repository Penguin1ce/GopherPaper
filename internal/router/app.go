// Package router 装配 gin 引擎与路由。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"GopherPaper/docs"
	"GopherPaper/internal/handler"
	adminhandler "GopherPaper/internal/handler/admin"
	chathandler "GopherPaper/internal/handler/chat"
	graphhandler "GopherPaper/internal/handler/graph"
	paperhandler "GopherPaper/internal/handler/paper"
	ssehandler "GopherPaper/internal/handler/sse"
	"GopherPaper/internal/handler/user"
	"GopherPaper/internal/middleware"
	"GopherPaper/internal/response"
	"GopherPaper/internal/zlog"
)

// Init 构建 gin 引擎。依赖必须在启动阶段完成初始化。
func Init(mode string) *gin.Engine {
	gin.SetMode(mode)
	docs.SwaggerInfo.BasePath = "/api/v1"

	r := gin.New()
	r.Use(gin.LoggerWithWriter(zlog.Writer()), gin.RecoveryWithWriter(zlog.Writer()))

	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 健康检查，无需鉴权。
	r.GET("/healthz", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	{
		api.POST("/auth/token", handler.Token)

		// 公开接口：用户注册登录与邮箱验证码。
		api.POST("/user/send-code", user.SendCode) // 下发邮箱验证码
		api.POST("/user/register", user.Register)  // 校验验证码并注册
		api.POST("/user/login", user.Login)        // 登录签发 JWT
		api.POST("/user/password-reset/send-code", user.SendPasswordResetCode)
		api.POST("/user/password-reset", user.ResetPassword)
		api.GET("/user/avatar-files/:name", user.AvatarFile)
		api.POST("/admin/send-code", adminhandler.SendCode)
		api.POST("/admin/register", adminhandler.Register)
		api.POST("/admin/login", adminhandler.Login)

		api.GET("/events", ssehandler.Subscribe)
		api.GET("/papers/:id/figures/:name", paperhandler.Figure)
		api.GET("/papers/:id/file", paperhandler.File)

		authed := api.Group("")
		authed.Use(middleware.JWTAuth())
		{
			authed.GET("/user/me", user.Me)
			authed.POST("/user/profile", user.UpdateProfile)
			authed.PATCH("/user/profile", user.UpdateProfile)
			authed.POST("/user/email", user.UpdateEmail)
			authed.GET("/user/preferences", user.Preferences)
			authed.POST("/user/preferences", user.UpdatePreferences)
			authed.POST("/user/logout", user.Logout)
			authed.POST("/user/avatar", user.UploadAvatar)
			authed.DELETE("/user/avatar", user.ClearAvatar)

			authed.POST("/papers", paperhandler.Upload)
			authed.GET("/papers", paperhandler.List)
			authed.GET("/papers/search", paperhandler.Search)
			authed.POST("/papers/compare", paperhandler.Compare)
			authed.GET("/papers/compare/status", paperhandler.CompareStatus)
			authed.GET("/papers/compare/reports", paperhandler.CompareReports)
			authed.DELETE("/papers/compare/reports/:report_id", paperhandler.DeleteCompareReport)
			authed.GET("/papers/:id", paperhandler.Detail)
			authed.DELETE("/papers/:id", paperhandler.Delete)
			authed.GET("/papers/:id/status", paperhandler.Status)
			authed.POST("/papers/:id/reparse", paperhandler.Reparse)
			authed.POST("/papers/:id/sections/rebuild", paperhandler.RebuildSections)
			authed.GET("/papers/:id/reports", paperhandler.Reports)
			authed.POST("/papers/:id/report", paperhandler.Report)
			authed.GET("/papers/:id/flow", paperhandler.GetFlow)
			authed.POST("/papers/:id/flow", paperhandler.Flow)
			authed.POST("/papers/:id/translate", paperhandler.Translate)
			authed.PATCH("/papers/:id/progress", paperhandler.UpdateProgress)
			authed.GET("/papers/:id/annotations", paperhandler.ListAnnotations)
			authed.POST("/papers/:id/annotations", paperhandler.CreateAnnotation)
			authed.PATCH("/papers/:id/annotations/:annotation_id", paperhandler.UpdateAnnotation)
			authed.DELETE("/papers/:id/annotations/:annotation_id", paperhandler.DeleteAnnotation)
			authed.POST("/papers/:id/mind-maps/build", paperhandler.BuildMindMap)
			authed.GET("/papers/:id/mind-maps", paperhandler.GetMindMap)
			authed.PUT("/mind-maps/:mind_map_id", paperhandler.UpdateMindMap)
			authed.POST("/mind-maps/:mind_map_id/sync", paperhandler.SyncMindMap)

			// 知识图谱：论文关系发现与研究趋势,按用户隔离。
			authed.GET("/graph/overview", graphhandler.Overview) // 图谱规模总览
			authed.GET("/graph/trends", graphhandler.Trends)     // 研究趋势:年度论文数与关键词热度
			authed.GET("/graph/keywords", graphhandler.Keywords) // 热门关键词
			authed.GET("/graph/network", graphhandler.Network)   // 总览知识图谱
			authed.GET("/graph/network/entities", graphhandler.NetworkEntities)
			authed.POST("/graph/network/rebuild", graphhandler.RebuildNetwork)
			authed.GET("/graph/network/rebuild/:job_id", graphhandler.RebuildNetworkStatus)
			authed.GET("/graph/papers/:id", graphhandler.PaperGraph) // 单篇论文知识图谱
			authed.POST("/graph/papers/:id/rebuild", graphhandler.RebuildPaper)
			authed.GET("/graph/papers/:id/related", graphhandler.Related) // 与某篇论文相关的论文

			authed.POST("/sessions", chathandler.CreateSession)
			authed.GET("/sessions", chathandler.ListSessions)
			authed.DELETE("/sessions/:id", chathandler.DeleteSession)
			authed.GET("/sessions/:id/messages", chathandler.ListMessages)
			authed.POST("/sessions/:id/messages", chathandler.SendMessage)
			authed.GET("/topics", chathandler.ListTopics)
			authed.POST("/topics/backfill", chathandler.BackfillTopics)
			authed.DELETE("/topics", chathandler.ClearTopics)
		}

		adminAuthed := api.Group("/admin")
		adminAuthed.Use(middleware.AdminJWTAuth())
		adminAuthed.Use(middleware.AdminAudit())
		{
			adminAuthed.GET("/me", adminhandler.Me)
			adminAuthed.GET("/overview", adminhandler.Overview)
			adminAuthed.GET("/analytics", adminhandler.Analytics)
			adminAuthed.GET("/health", adminhandler.Health)
			adminAuthed.GET("/activity", adminhandler.Activity)
			adminAuthed.GET("/papers", adminhandler.ListPapers)
			adminAuthed.DELETE("/papers/:id", adminhandler.DeletePaper)
			adminAuthed.GET("/model-configs", adminhandler.ListModelConfigs)
			adminAuthed.POST("/model-configs/apply", adminhandler.ApplyModelConfigs)
			adminAuthed.PUT("/model-configs/:role", adminhandler.UpdateModelConfig)
			adminAuthed.POST("/model-configs/:role/test", adminhandler.TestModelConfig)
			adminAuthed.POST("/model-configs/:role/restore", adminhandler.RestoreModelConfig)
			adminAuthed.POST("/demo/seed", adminhandler.SeedDemo)
			adminAuthed.POST("/demo/clear", adminhandler.ClearDemo)
			adminAuthed.GET("/users", adminhandler.ListUsers)
			adminAuthed.GET("/users/:id", adminhandler.GetUser)
			adminAuthed.GET("/classes", adminhandler.ClassStats)
			adminAuthed.GET("/logs", adminhandler.ListLogs)
			adminAuthed.GET("/logs/stats", adminhandler.LogStats)
			adminAuthed.GET("/analytics/advanced", adminhandler.AdvancedAnalytics)
			adminAuthed.GET("/audit", adminhandler.ListAudit)
			adminAuthed.GET("/export/papers", adminhandler.ExportPapersCSV)
			adminAuthed.GET("/export/users", adminhandler.ExportUsersCSV)
			adminAuthed.GET("/export/logs", adminhandler.ExportLogsCSV)
			adminAuthed.GET("/settings", adminhandler.ListSettings)
			adminAuthed.PUT("/settings", adminhandler.UpdateSettings)
			adminAuthed.GET("/announcements", adminhandler.ListAnnouncements)
			adminAuthed.POST("/announcements", adminhandler.CreateAnnouncement)
			adminAuthed.PUT("/announcements/:id", adminhandler.UpdateAnnouncement)
			adminAuthed.DELETE("/announcements/:id", adminhandler.DeleteAnnouncement)
			adminAuthed.GET("/feedbacks", adminhandler.ListFeedbacks)
			adminAuthed.PUT("/feedbacks/:id", adminhandler.UpdateFeedback)
			adminAuthed.GET("/tags", adminhandler.ListTags)
			adminAuthed.PUT("/tags/:id", adminhandler.RenameTag)
			adminAuthed.DELETE("/tags/:id", adminhandler.DeleteTag)
			adminAuthed.POST("/tags/merge", adminhandler.MergeTags)
			adminAuthed.GET("/sessions", adminhandler.ListSessions)
			adminAuthed.DELETE("/sessions/:id", adminhandler.DeleteSession)
			adminAuthed.GET("/admins", adminhandler.ListAdmins)
			adminAuthed.PUT("/admins/:id/status", adminhandler.SetAdminStatus)
			adminAuthed.POST("/papers/batch-delete", adminhandler.BatchDeletePapers)
			adminAuthed.GET("/storage", adminhandler.StorageOverview)

			// 运维任务看板
			adminAuthed.GET("/tasks", adminhandler.ListTasks)
			adminAuthed.POST("/tasks", adminhandler.CreateTask)
			adminAuthed.GET("/tasks/board", adminhandler.TaskBoard)
			adminAuthed.GET("/tasks/stats", adminhandler.TaskStats)
			adminAuthed.GET("/tasks/export", adminhandler.ExportTasksCSV)
			adminAuthed.POST("/tasks/bulk", adminhandler.BulkTasks)
			adminAuthed.POST("/tasks/demo/seed", adminhandler.SeedDemoTasks)
			adminAuthed.POST("/tasks/demo/clear", adminhandler.ClearDemoTasks)
			adminAuthed.GET("/tasks/:id", adminhandler.GetTaskDetail)
			adminAuthed.PUT("/tasks/:id", adminhandler.UpdateTask)
			adminAuthed.DELETE("/tasks/:id", adminhandler.DeleteTask)
			adminAuthed.POST("/tasks/:id/move", adminhandler.MoveTask)
			adminAuthed.GET("/tasks/:id/comments", adminhandler.ListTaskComments)
			adminAuthed.POST("/tasks/:id/comments", adminhandler.AddTaskComment)
			adminAuthed.DELETE("/tasks/:id/comments/:commentId", adminhandler.DeleteTaskComment)
			adminAuthed.GET("/tasks/:id/checklist", adminhandler.ListChecklist)
			adminAuthed.POST("/tasks/:id/checklist", adminhandler.AddChecklistItem)
			adminAuthed.PUT("/tasks/:id/checklist/:itemId", adminhandler.UpdateChecklistItem)
			adminAuthed.DELETE("/tasks/:id/checklist/:itemId", adminhandler.DeleteChecklistItem)
			adminAuthed.GET("/tasks/:id/activities", adminhandler.ListTaskActivities)
		}
	}
	return r
}
