// Package sse 处理 SSE 订阅,把解析进度与报告就绪实时推给前端。
// 浏览器 EventSource 无法在握手时带自定义头，鉴权走 query token，校验通过后再建流。
package sse

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/auth"
	"GopherPaper/internal/response"
	hub "GopherPaper/internal/sse"
)

// heartbeat 心跳间隔,定期发 SSE 注释行防代理或浏览器空闲断连。
const heartbeat = 25 * time.Second

// Subscribe 建立 SSE 流并按用户订阅解析进度与报告就绪推送。
// GET /api/v1/events?token=<jwt>
func Subscribe(c *gin.Context) {
	// SSE 允许跨源直连:dev 下前端绕开 Next 代理直接连本端口(:8080),避免代理 fetch
	// 长连断开不回收、堆满 Next 源连接池致前端卡死。鉴权走 query token、无 cookie,
	// 故只回显 Origin 放行即可(EventSource 简单 GET 不触发预检)。
	if origin := c.GetHeader("Origin"); origin != "" {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Vary", "Origin")
	}

	claims, err := auth.Parse(c.Query("token"))
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "token 无效")
		return
	}

	hub.WriteHeaders(c)

	sub := hub.Add(claims.StudentID)
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
