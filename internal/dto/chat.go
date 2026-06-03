// Package dto 定义 HTTP 层的请求与响应结构。
package dto

// ChatRequest 是助教对话请求体。
type ChatRequest struct {
	Query string `json:"query" binding:"required"`
}

// ChatResponse 是助教的统一响应，出题/批改也复用。
type ChatResponse struct {
	Intent  string         `json:"intent"`  // 命中的意图 concept/debug/review/exam/grade
	Content string         `json:"content"` // 文本回答
	Meta    map[string]any `json:"meta,omitempty"`
}

// ExamRequest 是出题请求体，由前端按钮带结构化参数提交。
type ExamRequest struct {
	Topic      string `json:"topic" binding:"required"` // 知识点
	Count      string `json:"count"`                    // 题量，缺省由 agent 兜底
	Difficulty string `json:"difficulty"`               // 难度，缺省由 agent 兜底
}

// GradeRequest 是批改请求体。Question 可选，Answer 为学生作答。
type GradeRequest struct {
	Question string `json:"question"`
	Answer   string `json:"answer" binding:"required"`
}

// Response 是统一响应信封。Code 为 0 表示成功，非 0 时与 HTTP 状态一致；
// Data 承载业务载荷，错误时省略。
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
