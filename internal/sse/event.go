package sse

import (
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
)

// Flusher 是 SSE 写出目标:可写且可逐条刷新(gin.ResponseWriter 满足)。
// 抽出接口让协议原语既服务推送中心订阅,也服务任意请求级流式 handler(如会话流式应答)。
type Flusher interface {
	io.Writer
	Flush()
}

// WriteHeaders 写 SSE 标准响应头,须在首次写事件前调用。
// 统一两类 SSE 端点(推送中心订阅与请求级流式应答)的头,避免各处漂移。
func WriteHeaders(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // 反代不缓冲,事件即发即达
}

// WriteEvent 写一条 SSE 事件并 flush。event 为空省略 event 行(前端 onmessage 收),
// 非空带 event 名(前端 addEventListener 按名收)。data 为已序列化的负载。
func WriteEvent(w Flusher, event, data string) error {
	var err error
	if event == "" {
		_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	} else {
		_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	}
	w.Flush()
	return err
}

// WriteComment 写一条 SSE 注释行(以 : 开头)并 flush,用于建流握手与心跳保活。
func WriteComment(w Flusher, text string) error {
	_, err := fmt.Fprintf(w, ": %s\n\n", text)
	w.Flush()
	return err
}
