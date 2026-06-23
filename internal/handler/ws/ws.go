// Package ws 处理 SSE 订阅,把解析进度与报告就绪实时推给前端。
// 浏览器 EventSource 无法在握手时带自定义头，鉴权走 query token，校验通过后再建流。
package ws

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/auth"
	"GopherPaper/internal/response"
	hub "GopherPaper/internal/ws"
)

// heartbeat 心跳间隔,定期发 SSE 注释行防代理或浏览器空闲断连。
const heartbeat = 25 * time.Second

// Subscribe 建立 SSE 流并按用户订阅解析进度与报告就绪推送。
// GET /api/v1/events?token=<jwt>
func Subscribe(c *gin.Context) {
	claims, err := auth.Parse(c.Query("token"))
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "token 无效")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 关中间层缓冲,保证逐条实时

	sub := hub.Add(claims.StudentID)
	defer hub.Remove(sub)

	// 先发一行注释建立流,前端 EventSource onopen 触发。
	fmt.Fprint(c.Writer, ": connected\n\n")
	c.Writer.Flush()

	ctx := c.Request.Context()
	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-sub.C():
			if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", msg); err != nil {
				return
			}
			c.Writer.Flush()
		case <-ticker.C:
			if _, err := fmt.Fprint(c.Writer, ": ping\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}
