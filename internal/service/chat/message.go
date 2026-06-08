package chat

import (
	"context"
	"fmt"
	"time"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
	chatdao "GopherPaper/internal/dao/chat"
	"GopherPaper/internal/history"
	"GopherPaper/internal/model"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// SendMessage 在会话内发一轮对话，返回助教消息与本轮引用出处。
// 多轮上下文与历史均由 trpc MySQL Session 承载，读最近若干条喂模型，应答后同步追加事件。
// 返回的助教消息 ID 为空，CreatedAt 为应答时刻，落 Session 后真实事件 ID 由 ListMessages 还原。
func SendMessage(ctx context.Context, studentID, sessionID, query string) (*model.Message, map[string]any, error) {
	start := time.Now()

	sess, err := ownedSession(ctx, studentID, sessionID)
	if err != nil {
		return nil, nil, err
	}
	// 会话绑定了论文时围绕该论文检索。
	ctx = core.WithPaperID(ctx, sess.PaperID)
	checkMS := time.Since(start).Milliseconds()

	step := time.Now()
	hist, err := history.Load(ctx, studentID, sessionID, constant.MaxContextMessages)
	if err != nil {
		return nil, nil, err
	}
	historyMS := time.Since(step).Milliseconds()

	step = time.Now()
	reply, err := ai.Chat(ctx, hist, query)
	if err != nil {
		return nil, nil, fmt.Errorf("service: 助教应答失败: %w", err)
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
	return aiMsg, reply.Meta, nil
}
