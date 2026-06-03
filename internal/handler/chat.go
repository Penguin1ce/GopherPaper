package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"GopherCPP/internal/ai"
	"GopherCPP/internal/dto"
	"GopherCPP/internal/response"
	"GopherCPP/internal/zlog"
)

// Chat 接收学生提问，由 Host 路由到答疑专家。
// POST /api/v1/chat
func Chat(c *gin.Context) {
	var req dto.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	// context 已由 JWT 中间件注入学生身份，RAG 检索依赖它。
	reply, err := ai.Chat(c.Request.Context(), req.Query)
	if err != nil {
		zlog.Error("assistant chat failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "处理失败")
		return
	}
	response.OK(c, reply2resp(reply))
}

// Exam 由前端出题按钮触发，带结构化参数直接出题。
// POST /api/v1/exam
func Exam(c *gin.Context) {
	var req dto.ExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	reply, err := ai.GenerateExam(c.Request.Context(), req.Topic, req.Count, req.Difficulty)
	if err != nil {
		zlog.Error("assistant exam failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "出题失败")
		return
	}
	response.OK(c, reply2resp(reply))
}

// Grade 由前端批改按钮触发，带题目与作答直接批改。
// POST /api/v1/grade
func Grade(c *gin.Context) {
	var req dto.GradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	reply, err := ai.Grade(c.Request.Context(), buildSubmission(req))
	if err != nil {
		zlog.Error("assistant grade failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "批改失败")
		return
	}
	response.OK(c, reply2resp(reply))
}

// buildSubmission 把题目与作答拼成批改输入。
func buildSubmission(req dto.GradeRequest) string {
	if req.Question == "" {
		return req.Answer
	}
	var b strings.Builder
	b.WriteString("题目：\n")
	b.WriteString(req.Question)
	b.WriteString("\n\n学生作答：\n")
	b.WriteString(req.Answer)
	return b.String()
}
