// Package dto 定义 HTTP 层的请求与响应结构。
package dto

import "GopherPaper/internal/model"

// CreateSessionRequest 创建会话，paper_id 与 title 可选。
type CreateSessionRequest struct {
	PaperID string `json:"paper_id"`
	Title   string `json:"title"`
}

// SendMessageRequest 会话内发一轮消息。
type SendMessageRequest struct {
	Query string `json:"query" binding:"required"`
}

// SendMessageResponse 返回助教消息与本轮引用出处。
type SendMessageResponse struct {
	Message *model.Message `json:"message"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// ChatResponse 是问答的统一响应，研读报告也复用。
type ChatResponse struct {
	Intent  string         `json:"intent"`  // 命中的问答子类 fact/summary/method
	Content string         `json:"content"` // 文本回答
	Meta    map[string]any `json:"meta,omitempty"`
}

// ReportRequest 是研读报告请求体，由前端按钮带报告类型触发。
type ReportRequest struct {
	Type string `json:"type" binding:"required"` // quickread/method/result/innovation/compare/future
}

// TranslateRequest 是精读页逐段翻译请求体，前端把选中的英文原文送来。
type TranslateRequest struct {
	Text string `json:"text" binding:"required"` // 待翻译的英文原文
}

// Response 是统一响应信封。Code 为 0 表示成功，非 0 时与 HTTP 状态一致；
// Data 承载业务载荷，错误时省略。
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
