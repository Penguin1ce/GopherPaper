// Package ws 是按用户分发的 WebSocket 推送中心，包级单例。
// 业务侧只调 PushStatus，不关心连接细节，连接的注册注销由 handler 负责。
package ws

import (
	"sync"

	"github.com/gorilla/websocket"

	"GopherPaper/internal/zlog"
)

// conns 按 userID 维护在线连接集合。
var (
	mu    sync.RWMutex
	conns = map[string]map[*websocket.Conn]struct{}{}
)

// StatusMessage 是推送给前端的解析进度消息。
type StatusMessage struct {
	Type    string `json:"type"` // 固定 "paper_status"
	PaperID string `json:"paper_id"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

// ReportMessage 是研读报告后台预生成完成的就绪通知,前端据此免轮询直接拉缓存。
type ReportMessage struct {
	Type       string `json:"type"` // 固定 "report_ready"
	PaperID    string `json:"paper_id"`
	ReportType string `json:"report_type"`
}

// Register 把某用户的一条连接登记进来。
func Register(userID string, c *websocket.Conn) {
	mu.Lock()
	defer mu.Unlock()
	set := conns[userID]
	if set == nil {
		set = map[*websocket.Conn]struct{}{}
		conns[userID] = set
	}
	set[c] = struct{}{}
}

// Unregister 注销某用户的一条连接，集合空了则删键。
func Unregister(userID string, c *websocket.Conn) {
	mu.Lock()
	defer mu.Unlock()
	set := conns[userID]
	if set == nil {
		return
	}
	delete(set, c)
	if len(set) == 0 {
		delete(conns, userID)
	}
}

// PushStatus 向某用户所有在线连接广播一条解析状态。
func PushStatus(userID, paperID, status, detail string) {
	broadcast(userID, paperID, StatusMessage{Type: "paper_status", PaperID: paperID, Status: status, Detail: detail})
}

// PushReport 向某用户所有在线连接广播某类研读报告已就绪。
func PushReport(userID, paperID, reportType string) {
	broadcast(userID, paperID, ReportMessage{Type: "report_ready", PaperID: paperID, ReportType: reportType})
}

// broadcast 把消息写给某用户的全部在线连接,快照连接集合后逐条发,避免持锁写。
func broadcast(userID, paperID string, msg any) {
	mu.RLock()
	targets := make([]*websocket.Conn, 0, len(conns[userID]))
	for c := range conns[userID] {
		targets = append(targets, c)
	}
	mu.RUnlock()
	for _, c := range targets {
		if err := c.WriteJSON(msg); err != nil {
			zlog.Warn("ws 推送失败", "user", userID, "paper", paperID, "err", err)
		}
	}
}
