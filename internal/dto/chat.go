// Package dto 定义 HTTP 层的请求与响应结构。
package dto

import "GopherPaper/internal/model"

// CreateSessionRequest 创建会话，paper_id 与 title 可选。
// agent_type 可选，空为默认论文助教，pioneer 为小云雀。
type CreateSessionRequest struct {
	PaperID   string `json:"paper_id"`
	Title     string `json:"title"`
	AgentType string `json:"agent_type"`
}

// SendMessageRequest 会话内发一轮消息。
type SendMessageRequest struct {
	Query string `json:"query" binding:"required"`
}

// SendMessageResponse 返回助教消息与本轮引用出处,SSE 下作为 done 事件载荷。
type SendMessageResponse struct {
	Message *model.Message `json:"message"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// 发消息 SSE 各事件的载荷。事件名见 constant.StreamEvent*,done 事件复用 SendMessageResponse。
type (
	// StreamToolPayload 是 tool_call / tool_result 事件载荷。
	StreamToolPayload struct {
		Tool string `json:"tool"`
	}
	// StreamDeltaPayload 是 delta 事件载荷,reset 为 true 时前端先清空已流式正文再追加。
	StreamDeltaPayload struct {
		Content string `json:"content"`
		Reset   bool   `json:"reset,omitempty"`
	}
	// StreamPlanPayload 是 plan 事件载荷,phase 标规划阶段,content 为该阶段文本增量。
	StreamPlanPayload struct {
		Phase   string `json:"phase"`
		Content string `json:"content"`
	}
	// StreamPaperDeleteConfirmPayload 是 confirm_delete_paper 事件载荷。
	StreamPaperDeleteConfirmPayload struct {
		PaperID           string `json:"paper_id"`
		Title             string `json:"title"`
		FileName          string `json:"file_name,omitempty"`
		Status            string `json:"status,omitempty"`
		ConfirmationToken string `json:"confirmation_token"`
		Message           string `json:"message,omitempty"`
	}
	// StreamErrorPayload 是 error 事件载荷。
	StreamErrorPayload struct {
		Message string `json:"message"`
	}
)

// ChatResponse 是问答的统一响应，研读报告也复用。
type ChatResponse struct {
	Intent  string         `json:"intent"`  // 命中的聊天意图 chitchat/summary/method/pioneer
	Content string         `json:"content"` // 文本回答
	Meta    map[string]any `json:"meta,omitempty"`
}

// ReportRequest 是研读报告请求体，由前端按钮带报告类型触发。
type ReportRequest struct {
	Type string `json:"type" binding:"required"` // quickread/method/result/innovation/future
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
