package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/toolkit"
	"GopherPaper/internal/ai/topic"
	chatdao "GopherPaper/internal/dao/chat"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/history"
	"GopherPaper/internal/model"
	"GopherPaper/internal/service/metrics"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

type SendMessageInput struct {
	Query                string
	DisplayContent       string
	ConfirmDeletePaperID string
	ReaderContext        *core.ReaderContext
}

// SendMessage 在会话内发一轮对话，返回助教消息与本轮引用出处。
// 多轮上下文与历史均由 trpc MySQL Session 承载，读最近若干条喂模型，应答后同步追加事件。
// 返回的助教消息 ID 为空，CreatedAt 为应答时刻，落 Session 后真实事件 ID 由 ListMessages 还原。
func SendMessage(ctx context.Context, studentID, sessionID string, in SendMessageInput) (*model.Message, map[string]any, error) {
	start := time.Now()
	metricSuccess := false
	var metricErr error
	metricPaperID := ""
	defer func() {
		metrics.Record(ctx, metrics.ServiceChat, studentID, metricPaperID, sessionID, metricSuccess, time.Since(start), metricErr)
	}()

	sess, err := ownedSession(ctx, studentID, sessionID)
	if err != nil {
		metricErr = err
		return nil, nil, err
	}
	query := strings.TrimSpace(in.Query)
	displayContent := strings.TrimSpace(in.DisplayContent)
	if displayContent == "" {
		displayContent = query
	}
	if sess.AgentType == constant.AgentMaodie && strings.TrimSpace(sess.PaperID) == "" {
		metricErr = errs.ErrPaperRequired
		return nil, nil, metricErr
	}
	// 会话绑定了论文时围绕该论文检索,并把标题注入 ctx 供问答链路写进 system prompt,
	// 消除「这篇论文」的指代悬空,防模型在检索前反问论文标识。
	ctx = core.WithPaperID(ctx, sess.PaperID)
	if title := boundPaperTitle(ctx, sess.PaperID); title != "" {
		ctx = core.WithPaperTitle(ctx, title)
	}
	ctx, steps := withExecutionRecorder(ctx)
	metricPaperID = sess.PaperID
	checkMS := time.Since(start).Milliseconds()

	step := time.Now()
	hist, err := history.Load(ctx, studentID, sessionID, constant.MaxContextMessages)
	if err != nil {
		metricErr = err
		return nil, nil, err
	}
	shouldRewriteTitle := shouldRewriteSessionTitle(sess, hist)
	historyMS := time.Since(step).Milliseconds()

	step = time.Now()
	// 小云雀/小耄耋会话各走独立 agent;其余走默认论文问答链路。
	var reply *core.Reply
	if sess.AgentType == constant.AgentPioneer && strings.TrimSpace(in.ConfirmDeletePaperID) != "" {
		reply, err = confirmPioneerPaperDelete(ctx, in.ConfirmDeletePaperID)
	} else if sess.AgentType == constant.AgentPioneer {
		// 小云雀跨轮记忆由 runner 的 Redis session 承载,传 sessionID 即可,无需注入文本历史。
		reply, err = ai.PioneerChat(ctx, sessionID, query)
	} else if sess.AgentType == constant.AgentMaodie {
		rc := core.ReaderContext{}
		if in.ReaderContext != nil {
			rc = *in.ReaderContext
		}
		reply, err = ai.MaodieChat(ctx, hist, query, rc)
	} else {
		reply, err = ai.Chat(ctx, hist, query)
	}
	if err != nil {
		metricErr = fmt.Errorf("service: 助教应答失败: %w", err)
		return nil, nil, metricErr
	}
	if savedSteps := steps.Steps(); len(savedSteps) > 0 {
		if reply.Meta == nil {
			reply.Meta = map[string]any{}
		}
		reply.Meta[constant.MetaKeyExecutionSteps] = savedSteps
	}
	aiMS := time.Since(step).Milliseconds()

	step = time.Now()
	now := time.Now()
	userMsg := &model.Message{SessionID: sessionID, Role: model.RoleUser, Content: displayContent, CreatedAt: now}
	aiMsg := &model.Message{SessionID: sessionID, Role: model.RoleAssistant, Content: reply.Content, Intent: reply.Intent, Meta: reply.Meta, CreatedAt: now}

	// 默认问答历史追加失败时沿用 best-effort;小云雀另有 Redis 工作记忆,
	// 持久化失败必须清掉本会话工作记忆并报错,避免出现用户看不见的隐形上下文。
	if err := history.Append(ctx, studentID, sessionID, userMsg, aiMsg); err != nil {
		zlog.Error("追加会话历史失败", "session_id", sessionID, "err", err)
		if sess.AgentType == constant.AgentPioneer {
			if clearErr := ai.DeletePioneerSessionMemory(ctx, studentID, sessionID); clearErr != nil {
				zlog.Warn("清理小云雀工作记忆失败", "session_id", sessionID, "err", clearErr)
			}
			metricErr = fmt.Errorf("service: 追加会话历史失败: %w", err)
			return nil, nil, metricErr
		}
	}
	_ = chatdao.TouchSession(ctx, sessionID) // 刷新列表排序，失败不影响应答
	persistMS := time.Since(step).Milliseconds()

	titleMS := int64(0)
	if shouldRewriteTitle {
		titleStep := time.Now()
		rewriteAndSaveSessionTitle(ctx, studentID, sessionID, displayContent)
		titleMS = time.Since(titleStep).Milliseconds()
	}

	// 小云雀会话异步按对话内容归类,不阻塞应答。首轮必触发;前几轮内容变厚时再触发让 Classify 重排
	// (hist 为本轮前历史,前 TopicRefineMaxTurns 轮内放行,之后锁定省去无谓 goroutine)。
	// Classify 内按会话串行并再校验轮数,防并发与超窗重排。
	if sess.AgentType == constant.AgentPioneer &&
		(sess.TopicID == "" || len(hist) < constant.TopicRefineMaxTurns*2) {
		go func() {
			bg := context.WithoutCancel(ctx) // 保住 tenant 身份又不被请求 ctx 取消
			if err := topic.Classify(bg, studentID, sessionID); err != nil {
				zlog.Warn("会话主题归类失败", "session_id", sessionID, "err", err)
			}
		}()
	}

	// 分阶段耗时，ai_ms 通常是大头，persist_ms 为历史追加耗时。
	zlog.Info("发消息完成",
		"session_id", sessionID,
		"student_id", studentID,
		"intent", reply.Intent,
		"history_n", len(hist),
		"check_ms", checkMS,
		"history_ms", historyMS,
		"ai_ms", aiMS,
		"persist_ms", persistMS,
		"title_ms", titleMS,
		"total_ms", time.Since(start).Milliseconds(),
	)
	metricSuccess = true
	return aiMsg, reply.Meta, nil
}

func confirmPioneerPaperDelete(ctx context.Context, paperID string) (*core.Reply, error) {
	title, message, err := toolkit.ConfirmPaperDelete(ctx, paperID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(message) == "" {
		message = fmt.Sprintf("已删除《%s》。", title)
	}
	return &core.Reply{Intent: constant.IntentPioneer, Content: message}, nil
}

// boundPaperTitle 查会话绑定论文的标题,抽取未完成时退回文件名,查库失败只降级不阻断问答。
func boundPaperTitle(ctx context.Context, paperID string) string {
	if strings.TrimSpace(paperID) == "" {
		return ""
	}
	p, err := paperdao.Get(ctx, paperID)
	if err != nil {
		zlog.Warn("查会话绑定论文失败", "paper_id", paperID, "err", err)
		return ""
	}
	if title := strings.TrimSpace(p.Title); title != "" {
		return title
	}
	return strings.TrimSpace(p.FileName)
}
