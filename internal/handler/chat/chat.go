// Package chat 处理会话与消息的 HTTP 接口。
// 处理函数为裸包级 func，业务委托 service，本层只做参数绑定、身份取用与错误映射。
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/toolkit"
	"GopherPaper/internal/credential"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/response"
	chatservice "GopherPaper/internal/service/chat"
	"GopherPaper/internal/sse"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

const sendMessageStreamBuffer = 256

// CreateSession 新建会话。
// POST /api/v1/sessions
//
// @Summary 创建会话
// @Description 创建一段多轮会话，可绑定论文，也可指定 agent_type=pioneer 使用小云雀、agent_type=maodie 使用小耄耋。
// @Tags sessions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateSessionRequest true "会话参数"
// @Success 200 {object} dto.Response{data=model.Session}
// @Failure 400 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /sessions [post]
func CreateSession(c *gin.Context) {
	var req dto.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	studentID := tenant.MustStudentID(c.Request.Context())
	s, err := chatservice.CreateSession(c.Request.Context(), studentID, req.PaperID, req.Title, req.AgentType)
	if err != nil {
		if errors.Is(err, errs.ErrAgentTypeInvalid) || errors.Is(err, errs.ErrPaperRequired) {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		zlog.Error("创建会话失败", "student_id", studentID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "创建会话失败")
		return
	}
	response.OK(c, s)
}

// ListSessions 列出当前学生的会话。
// GET /api/v1/sessions
//
// @Summary 列出会话
// @Description 返回当前登录用户的会话列表。
// @Tags sessions
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.Response{data=[]model.Session}
// @Failure 500 {object} dto.Response
// @Router /sessions [get]
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

// ListTopics 列出当前学生的小云雀会话主题。
// GET /api/v1/topics
//
// @Summary 列出会话主题
// @Description 返回当前登录用户的小云雀会话主题，供侧边栏按主题分组渲染。
// @Tags sessions
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.Response{data=[]model.Topic}
// @Failure 500 {object} dto.Response
// @Router /topics [get]
func ListTopics(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	topics, err := chatservice.ListPioneerTopics(c.Request.Context(), studentID)
	if err != nil {
		zlog.Error("查询会话主题失败", "student_id", studentID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询主题失败")
		return
	}
	response.OK(c, topics)
}

// BackfillTopics 手动触发当前学生存量小云雀会话的主题回填。
// POST /api/v1/topics/backfill
//
// @Summary 回填会话主题
// @Description 把当前用户尚未归类的小云雀会话异步归类,返回待处理会话数。供上线前的存量会话补归类。
// @Tags sessions
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.Response
// @Failure 409 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /topics/backfill [post]
func BackfillTopics(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	n, err := chatservice.BackfillPioneerTopics(c.Request.Context(), studentID)
	if err != nil {
		if errors.Is(err, errs.ErrTopicBackfillBusy) {
			response.Fail(c, http.StatusConflict, err.Error())
			return
		}
		zlog.Error("回填会话主题失败", "student_id", studentID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "回填主题失败")
		return
	}
	response.OKMsg(c, "已开始整理历史会话", gin.H{"count": n})
}

// ClearTopics 清空当前学生的全部小云雀主题,会话退回未归类。供演示重置归类。
// DELETE /api/v1/topics
//
// @Summary 清空会话主题
// @Description 删除当前用户的全部小云雀主题并把会话退回未归类,返回删除的主题数。
// @Tags sessions
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /topics [delete]
func ClearTopics(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	n, err := chatservice.ClearPioneerTopics(c.Request.Context(), studentID)
	if err != nil {
		zlog.Error("清空会话主题失败", "student_id", studentID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "清空主题失败")
		return
	}
	response.OKMsg(c, "已清除主题归类", gin.H{"removed": n})
}

// DeleteSession 删除会话。
// DELETE /api/v1/sessions/:id
//
// @Summary 删除会话
// @Description 删除当前用户拥有的会话。
// @Tags sessions
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 ID"
// @Success 200 {object} dto.Response
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /sessions/{id} [delete]
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
//
// @Summary 拉取会话消息
// @Description 按时间升序返回某个会话的历史消息。
// @Tags sessions
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 ID"
// @Success 200 {object} dto.Response{data=[]model.Message}
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /sessions/{id}/messages [get]
func ListMessages(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	msgs, err := chatservice.ListMessages(c.Request.Context(), studentID, c.Param("id"))
	if err != nil {
		writeChatErr(c, err, "查询消息失败")
		return
	}
	response.OK(c, msgs)
}

// SendMessage 在会话内发一轮消息,以 SSE 推送生成过程:工具调用与文本增量实时下发,
// done 事件收尾带完整助教消息与引用出处。开流前的错误(参数/会话校验)仍走普通 JSON 状态码,
// 开流后的失败降级为 error 事件。
// POST /api/v1/sessions/:id/messages
//
// @Summary 发送会话消息
// @Description 在会话内发送一轮消息，以 SSE 推送 tool_call、tool_result、plan、delta、done、error 等事件；done 事件载荷为 dto.SendMessageResponse。
// @Tags sessions
// @Accept json
// @Produce text/event-stream
// @Security BearerAuth
// @Param id path string true "会话 ID"
// @Param X-Luckin-Token header string false "小云雀调用瑞幸工具时透传的凭据"
// @Param X-GopherPaper-Delete-Confirm header string false "论文删除确认令牌"
// @Param request body dto.SendMessageRequest true "消息内容"
// @Success 200 {string} string "SSE 事件流"
// @Failure 400 {object} dto.Response
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /sessions/{id}/messages [post]
func SendMessage(c *gin.Context) {
	var req dto.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	ctx := c.Request.Context()
	// 凭据型工具的 token 由前端保存、随请求头透传,只进 ctx 供工具调用注入,不落库不打日志。
	if tok := strings.TrimSpace(c.GetHeader(constant.HeaderLuckinToken)); tok != "" {
		ctx = credential.With(ctx, constant.CredentialLuckin, tok)
	}
	if tok := strings.TrimSpace(c.GetHeader(constant.HeaderPaperDeleteConfirm)); tok != "" {
		ctx = toolkit.WithPaperDeleteConfirmation(ctx, tok)
	}
	studentID := tenant.MustStudentID(ctx)
	sessionID := c.Param("id")

	// SSE 头在首个事件时才写,此前的错误仍能返回普通 JSON 状态码。
	started := false
	emit := func(name string, payload any) bool {
		if !started {
			sse.WriteHeaders(c)
			c.Writer.WriteHeader(http.StatusOK)
			started = true
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return true
		}
		return sse.WriteEvent(c.Writer, name, string(b)) == nil
	}

	events := make(chan core.StreamEvent, sendMessageStreamBuffer)
	resultCh := make(chan dto.SendMessageResponse, 1)
	errCh := make(chan error, 1)
	clientGone := make(chan struct{})
	defer close(clientGone)

	// 生成上下文脱离 HTTP 连接生命周期:用户切页、切会话或关闭浏览器时,
	// 前端流会断开,但本轮 agent 继续跑完并落库;流式事件只在连接仍在时转发。
	runCtx := context.WithoutCancel(ctx)
	runCtx = core.WithStream(runCtx, func(ev core.StreamEvent) {
		select {
		case events <- ev:
		case <-clientGone:
		}
	})
	go func() {
		msg, meta, err := chatservice.SendMessage(runCtx, studentID, sessionID, chatservice.SendMessageInput{
			Query:                req.Query,
			DisplayContent:       req.DisplayContent,
			ConfirmDeletePaperID: req.ConfirmDeletePaperID,
			ReaderContext:        req.ReaderContext,
		})
		close(events)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- dto.SendMessageResponse{Message: msg, Meta: meta}
	}()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case ev, ok := <-events:
			if !ok {
				select {
				case err := <-errCh:
					if !started {
						writeChatErr(c, err, "处理失败")
						return
					}
					zlog.Error("会话流式应答失败", "session_id", sessionID, "err", err)
					emit(constant.StreamEventError, dto.StreamErrorPayload{Message: "处理失败"})
					return
				case result := <-resultCh:
					emit(constant.StreamEventDone, result)
					return
				case <-c.Request.Context().Done():
					return
				}
			}
			if !emitStreamMessage(emit, ev) {
				return
			}
		}
	}
}

func emitStreamMessage(emit func(string, any) bool, ev core.StreamEvent) bool {
	switch ev.Kind {
	case constant.StreamEventDelta:
		return emit(ev.Kind, dto.StreamDeltaPayload{Content: ev.Delta, Reset: ev.Reset})
	case constant.StreamEventPlan:
		return emit(ev.Kind, dto.StreamPlanPayload{Phase: ev.Phase, Content: ev.Delta})
	case constant.StreamEventConfirmDeletePaper, constant.StreamEventPaperFlow, constant.StreamEventPaperFlowNode:
		return emit(ev.Kind, ev.Payload)
	default:
		// 原始工具名换前端显示名,未配置回退原始名。
		return emit(ev.Kind, dto.StreamToolPayload{Tool: toolkit.DisplayName(ev.Tool)})
	}
}

// writeChatErr 把会话错误映射为对应 HTTP 状态。
func writeChatErr(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, errs.ErrSessionNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, errs.ErrSessionForbidden):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, errs.ErrPaperRequired):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		zlog.Error("会话接口错误", "err", err)
		response.Fail(c, http.StatusInternalServerError, fallback)
	}
}
