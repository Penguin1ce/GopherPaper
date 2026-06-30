package chat

import (
	"context"
	"fmt"
	"time"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/topic"
	chatdao "GopherPaper/internal/dao/chat"
	"GopherPaper/internal/history"
	"GopherPaper/internal/model"
	"GopherPaper/internal/service/metrics"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// SendMessage 在会话内发一轮对话，返回助教消息与本轮引用出处。
// 多轮上下文与历史均由 trpc MySQL Session 承载，读最近若干条喂模型，应答后同步追加事件。
// 返回的助教消息 ID 为空，CreatedAt 为应答时刻，落 Session 后真实事件 ID 由 ListMessages 还原。
func SendMessage(ctx context.Context, studentID, sessionID, query string) (*model.Message, map[string]any, error) {
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
	// 会话绑定了论文时围绕该论文检索。
	ctx = core.WithPaperID(ctx, sess.PaperID)
	ctx, steps := withExecutionRecorder(ctx)
	metricPaperID = sess.PaperID
	checkMS := time.Since(start).Milliseconds()

	step := time.Now()
	hist, err := history.Load(ctx, studentID, sessionID, constant.MaxContextMessages)
	if err != nil {
		metricErr = err
		return nil, nil, err
	}
	historyMS := time.Since(step).Milliseconds()

	step = time.Now()
	// 小云雀会话不经意图分类与 RAG,直接走带工具 agent;其余走论文问答链路。
	var reply *core.Reply
	if sess.AgentType == constant.AgentPioneer {
		// 小云雀跨轮记忆由 runner 的 Redis session 承载,传 sessionID 即可,无需注入文本历史。
		reply, err = ai.PioneerChat(ctx, sessionID, query)
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
	userMsg := &model.Message{SessionID: sessionID, Role: model.RoleUser, Content: query, CreatedAt: now}
	aiMsg := &model.Message{SessionID: sessionID, Role: model.RoleAssistant, Content: reply.Content, Intent: reply.Intent, Meta: reply.Meta, CreatedAt: now}

	// 追加进 Session，失败不阻断应答，仅丢失本轮历史。
	if err := history.Append(ctx, studentID, sessionID, userMsg, aiMsg); err != nil {
		zlog.Error("追加会话历史失败", "session_id", sessionID, "err", err)
	}
	_ = chatdao.TouchSession(ctx, sessionID) // 刷新列表排序，失败不影响应答
	persistMS := time.Since(step).Milliseconds()

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
		"total_ms", time.Since(start).Milliseconds(),
	)
	metricSuccess = true
	return aiMsg, reply.Meta, nil
}
