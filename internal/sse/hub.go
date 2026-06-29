// Package sse 是按用户分发的服务端推送中心，包级单例，走 SSE(Server-Sent Events)。
// 业务侧只调 PushStatus/PushReport，不关心连接细节；订阅的注册注销由 handler 负责。
// 选 SSE 不选 WebSocket:前端经 Next 的 fetch 代理同源转发,而 fetch 路由无法完成 WS
// 升级握手,SSE 则能原样流式透传,免独立 origin 与 nginx Upgrade 规则。
package sse

import (
	"encoding/json"
	"sync"

	"GopherPaper/internal/zlog"
)

// subEventBuffer 单个订阅的事件缓冲;慢客户端塞满则丢最新事件,状态可经前端轮询兜底对账。
const subEventBuffer = 16

// Subscriber 是某用户的一条 SSE 订阅,handler 从 C() 读事件并下发。
type Subscriber struct {
	userID string
	ch     chan []byte
}

// C 暴露事件通道(只读),每条为已序列化的 JSON 消息。
func (s *Subscriber) C() <-chan []byte { return s.ch }

// subs 按 userID 维护在线订阅集合。
var (
	mu   sync.RWMutex
	subs = map[string]map[*Subscriber]struct{}{}
)

// StatusMessage 是推送给前端的解析进度消息。
type StatusMessage struct {
	Type          string `json:"type"` // 固定 "paper_status"
	PaperID       string `json:"paper_id"`
	Status        string `json:"status"`
	Detail        string `json:"detail,omitempty"`
	ParseProgress int    `json:"parse_progress,omitempty"`
	ParsedPages   int    `json:"parsed_pages,omitempty"`
	TotalPages    int    `json:"total_pages,omitempty"`
}

// ReportMessage 是研读报告生成完成的就绪通知,前端据此免轮询直接拉缓存。
type ReportMessage struct {
	Type       string `json:"type"` // 固定 "report_ready"
	PaperID    string `json:"paper_id"`
	ReportType string `json:"report_type"`
}

// ReportProgressMessage 是研读报告生成过程的阶段进度,前端据此在报告卡上显示执行计划
// (规划→撰写→评审)与失败态。Phase 取 constant.ReportPhase*。
type ReportProgressMessage struct {
	Type       string `json:"type"` // 固定 "report_progress"
	PaperID    string `json:"paper_id"`
	ReportType string `json:"report_type"`
	Phase      string `json:"phase"`
	Detail     string `json:"detail,omitempty"`
}

// Add 登记某用户的一条订阅并返回它,handler 退出时须 Remove。
func Add(userID string) *Subscriber {
	s := &Subscriber{userID: userID, ch: make(chan []byte, subEventBuffer)}
	mu.Lock()
	defer mu.Unlock()
	set := subs[userID]
	if set == nil {
		set = map[*Subscriber]struct{}{}
		subs[userID] = set
	}
	set[s] = struct{}{}
	return s
}

// Remove 注销一条订阅，集合空了则删键。不关闭通道,避免与并发广播的发送竞态。
func Remove(s *Subscriber) {
	mu.Lock()
	defer mu.Unlock()
	set := subs[s.userID]
	if set == nil {
		return
	}
	delete(set, s)
	if len(set) == 0 {
		delete(subs, s.userID)
	}
}

// PushStatus 向某用户所有在线订阅广播一条解析状态。
func PushStatus(userID, paperID, status, detail string) {
	broadcast(userID, paperID, StatusMessage{Type: "paper_status", PaperID: paperID, Status: status, Detail: detail})
}

// PushStatusProgress broadcasts MinerU page-level parse progress.
func PushStatusProgress(userID, paperID, status, detail string, parseProgress, parsedPages, totalPages int) {
	broadcast(userID, paperID, StatusMessage{
		Type: "paper_status", PaperID: paperID, Status: status, Detail: detail,
		ParseProgress: parseProgress, ParsedPages: parsedPages, TotalPages: totalPages,
	})
}

// PushReport 向某用户所有在线订阅广播某类研读报告已就绪。
func PushReport(userID, paperID, reportType string) {
	broadcast(userID, paperID, ReportMessage{Type: "report_ready", PaperID: paperID, ReportType: reportType})
}

// PushReportProgress 向某用户广播某类研读报告的生成阶段进度,供前端报告卡显示执行计划与失败态。
func PushReportProgress(userID, paperID, reportType, phase, detail string) {
	broadcast(userID, paperID, ReportProgressMessage{
		Type: "report_progress", PaperID: paperID, ReportType: reportType, Phase: phase, Detail: detail,
	})
}

// broadcast 把消息序列化后非阻塞地发给某用户的全部在线订阅,快照集合后逐条发,避免持锁写。
func broadcast(userID, paperID string, msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		zlog.Warn("sse 消息序列化失败", "user", userID, "paper", paperID, "err", err)
		return
	}
	mu.RLock()
	targets := make([]*Subscriber, 0, len(subs[userID]))
	for s := range subs[userID] {
		targets = append(targets, s)
	}
	mu.RUnlock()
	for _, s := range targets {
		select {
		case s.ch <- data:
		default:
			zlog.Warn("sse 订阅缓冲已满,丢弃事件", "user", userID, "paper", paperID)
		}
	}
}
