// Package sse 处理 SSE 订阅,把解析进度与报告就绪实时推给前端。
// 浏览器 EventSource 无法在握手时带自定义头，鉴权走短期一次性票据。
package sse

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/auth"
	"GopherPaper/internal/response"
	hub "GopherPaper/internal/sse"
	"GopherPaper/internal/tenant"
)

// heartbeat 心跳间隔,定期发 SSE 注释行防代理或浏览器空闲断连。
const heartbeat = 25 * time.Second

// Subscribe 建立 SSE 流并按用户订阅解析进度与报告就绪推送。
// GET /api/v1/events?ticket=<ticket>
//
// @Summary 订阅事件流
// @Description 建立 SSE 长连接，推送论文解析进度与报告就绪事件。票据须先通过受保护接口获取，30 秒内只可消费一次。
// @Tags events
// @Produce text/event-stream
// @Param ticket query string true "短期一次性票据"
// @Success 200 {string} string "SSE 事件流"
// @Failure 401 {object} dto.Response
// @Router /events [get]
func Subscribe(c *gin.Context) {
	// SSE 允许跨源直连:dev 下前端绕开 Next 代理直接连本端口(:8080),避免代理 fetch
	// 长连断开不回收、堆满 Next 源连接池致前端卡死。鉴权走 URL 中的一次性票据、无 cookie,
	// 故只回显 Origin 放行即可(EventSource 简单 GET 不触发预检)。
	if origin := c.GetHeader("Origin"); origin != "" {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Vary", "Origin")
	}

	studentID, err := auth.ConsumeSSETicket(c.Request.Context(), c.Query("ticket"))
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "SSE 票据无效或已过期")
		return
	}

	hub.WriteHeaders(c)

	sub := hub.Add(studentID)
	defer hub.Remove(sub)

	// 先发一行注释建立流,前端 EventSource onopen 触发。
	hub.WriteComment(c.Writer, "connected")

	ctx := c.Request.Context()
	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-sub.C():
			if err := hub.WriteEvent(c.Writer, "", string(msg)); err != nil {
				return
			}
		case <-ticker.C:
			if err := hub.WriteComment(c.Writer, "ping"); err != nil {
				return
			}
		}
	}
}

// Ticket 为当前用户签发 30 秒有效、只能消费一次的 SSE 连接票据。
// POST /api/v1/events/ticket
//
// @Summary 获取 SSE 一次性票据
// @Description 为当前登录用户签发 30 秒有效且只能消费一次的 SSE 连接票据。
// @Tags events
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.Response
// @Failure 401 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /events/ticket [post]
func Ticket(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	ticket, err := auth.MintSSETicket(c.Request.Context(), studentID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "创建 SSE 票据失败")
		return
	}
	c.Header("Cache-Control", "no-store")
	response.OK(c, gin.H{"ticket": ticket, "expires_in": int(auth.SSETicketTTL.Seconds())})
}
