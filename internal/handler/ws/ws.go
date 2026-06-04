// Package ws 处理 WebSocket 升级与解析进度订阅。
// 浏览器无法在握手时带自定义头，鉴权走 query token，校验通过后再升级。
package ws

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"GopherPaper/internal/auth"
	"GopherPaper/internal/response"
	hub "GopherPaper/internal/ws"
	"GopherPaper/internal/zlog"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Subscribe 升级连接并按用户订阅解析进度推送。
// GET /api/v1/ws?token=<jwt>
func Subscribe(c *gin.Context) {
	claims, err := auth.Parse(c.Query("token"))
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "token 无效")
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zlog.Warn("ws 升级失败", "err", err)
		return
	}
	hub.Register(claims.StudentID, conn)
	defer func() {
		hub.Unregister(claims.StudentID, conn)
		_ = conn.Close()
	}()
	// 阻塞读直到客户端断开，期间服务端经 hub 主动推送。
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
