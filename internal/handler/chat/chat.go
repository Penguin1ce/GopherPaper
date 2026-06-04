// Package chat 处理会话与消息的 HTTP 接口。
// 处理函数为裸包级 func，业务委托 service，本层只做参数绑定、身份取用与错误映射。
package chat

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/dto"
	"GopherPaper/internal/response"
	chatservice "GopherPaper/internal/service/chat"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/errs"
)

// CreateSession 新建会话。
// POST /api/v1/sessions
func CreateSession(c *gin.Context) {
	var req dto.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	studentID := tenant.MustStudentID(c.Request.Context())
	s, err := chatservice.CreateSession(c.Request.Context(), studentID, req.PaperID, req.Title)
	if err != nil {
		zlog.Error("创建会话失败", "student_id", studentID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "创建会话失败")
		return
	}
	response.OK(c, s)
}

// ListSessions 列出当前学生的会话。
// GET /api/v1/sessions
func ListSessions(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	sessions, err := chatservice.ListSessions(c.Request.Context(), studentID)
	if err != nil {
		zlog.Error("查询会话列表失败", "student_id", studentID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询会话失败")
		return
	}
	response.OK(c, sessions)
}

// DeleteSession 删除会话。
// DELETE /api/v1/sessions/:id
func DeleteSession(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	if err := chatservice.DeleteSession(c.Request.Context(), studentID, c.Param("id")); err != nil {
		writeChatErr(c, err, "删除会话失败")
		return
	}
	response.OKMsg(c, "已删除", nil)
}

// ListMessages 拉取会话历史消息，按时间升序还原上下文。
// GET /api/v1/sessions/:id/messages
func ListMessages(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	msgs, err := chatservice.ListMessages(c.Request.Context(), studentID, c.Param("id"))
	if err != nil {
		writeChatErr(c, err, "查询消息失败")
		return
	}
	response.OK(c, msgs)
}

// SendMessage 在会话内发一轮消息，返回助教应答与引用出处。
// POST /api/v1/sessions/:id/messages
func SendMessage(c *gin.Context) {
	var req dto.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	studentID := tenant.MustStudentID(c.Request.Context())
	msg, meta, err := chatservice.SendMessage(c.Request.Context(), studentID, c.Param("id"), req.Query)
	if err != nil {
		writeChatErr(c, err, "处理失败")
		return
	}
	response.OK(c, dto.SendMessageResponse{Message: msg, Meta: meta})
}

// writeChatErr 把会话错误映射为对应 HTTP 状态。
func writeChatErr(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, errs.ErrSessionNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, errs.ErrSessionForbidden):
		response.Fail(c, http.StatusForbidden, err.Error())
	default:
		zlog.Error("会话接口错误", "err", err)
		response.Fail(c, http.StatusInternalServerError, fallback)
	}
}
