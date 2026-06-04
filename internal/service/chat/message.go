package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"

	"GopherPaper/internal/agent"
	"GopherPaper/internal/ai"
	chatdao "GopherPaper/internal/dao/chat"
	"GopherPaper/internal/model"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// SendMessage 在会话内发一轮对话，返回助教消息与本轮引用出处。
// 上下文优先读 Redis 缓存，应答后同步写缓存保证多轮可见，再投递队列异步落库。
// 返回的助教消息 ID 为 0、CreatedAt 为应答时刻，真实 ID 待消费者落库后由 ListMessages 还原。
func SendMessage(ctx context.Context, studentID, sessionID, query string) (*model.Message, map[string]any, error) {
	start := time.Now()

	sess, err := ownedSession(ctx, studentID, sessionID)
	if err != nil {
		return nil, nil, err
	}
	// 会话绑定了论文时围绕该论文检索。
	ctx = agent.WithPaperID(ctx, sess.PaperID)
	checkMS := time.Since(start).Milliseconds()

	step := time.Now()
	history, err := recentContext(ctx, sessionID, constant.MaxContextMessages)
	if err != nil {
		return nil, nil, err
	}
	historyMS := time.Since(step).Milliseconds()

	step = time.Now()
	reply, err := ai.Chat(ctx, toSchemaMessages(history), query)
	if err != nil {
		return nil, nil, fmt.Errorf("service: 助教应答失败: %w", err)
	}
	aiMS := time.Since(step).Milliseconds()

	step = time.Now()
	now := time.Now()
	userMsg := &model.Message{SessionID: sessionID, Role: model.RoleUser, Content: query, CreatedAt: now}
	aiMsg := &model.Message{SessionID: sessionID, Role: model.RoleAssistant, Content: reply.Content, Intent: reply.Intent, CreatedAt: now}
	pending := []*model.Message{userMsg, aiMsg}

	// 先写缓存让下一轮多轮上下文立即可见，再投递入库队列。
	if err := chatdao.CacheAppendMessages(ctx, sessionID, pending); err != nil {
		zlog.Error("写入上下文缓存失败", "session_id", sessionID, "err", err)
	}
	if err := publishPersist(ctx, sessionID, pending); err != nil {
		// 投递失败兜底同步落库，保证消息不丢。
		zlog.Error("投递入库队列失败，改同步落库", "session_id", sessionID, "err", err)
		if dbErr := chatdao.CreateMessages(ctx, pending); dbErr != nil {
			return nil, nil, dbErr
		}
		_ = chatdao.TouchSession(ctx, sessionID)
	}
	persistMS := time.Since(step).Milliseconds()

	// 分阶段耗时，ai_ms 通常是大头；入库已异步，persist_ms 仅含写缓存与投递。
	zlog.Info("发消息完成",
		"session_id", sessionID,
		"student_id", studentID,
		"intent", reply.Intent,
		"history_n", len(history),
		"check_ms", checkMS,
		"history_ms", historyMS,
		"ai_ms", aiMS,
		"persist_ms", persistMS,
		"total_ms", time.Since(start).Milliseconds(),
	)
	return aiMsg, reply.Meta, nil
}

// recentContext 取会话最近上下文，优先读 Redis 缓存，未命中或缓存故障回落 MySQL 并回填。
func recentContext(ctx context.Context, sessionID string, limit int) ([]model.Message, error) {
	msgs, hit, err := chatdao.CacheRecentMessages(ctx, sessionID, limit)
	if err != nil {
		zlog.Error("读上下文缓存失败，回落 MySQL", "session_id", sessionID, "err", err)
	} else if hit {
		return msgs, nil
	}

	msgs, err = chatdao.RecentMessages(ctx, sessionID, limit)
	if err != nil {
		return nil, err
	}
	if err := chatdao.WarmContext(ctx, sessionID, msgs); err != nil {
		zlog.Error("回填上下文缓存失败", "session_id", sessionID, "err", err)
	}
	return msgs, nil
}

// toSchemaMessages 把库里的历史消息转成 eino 消息，喂给多 agent。
func toSchemaMessages(msgs []model.Message) []*schema.Message {
	out := make([]*schema.Message, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case model.RoleUser:
			out = append(out, schema.UserMessage(m.Content))
		case model.RoleAssistant:
			out = append(out, schema.AssistantMessage(m.Content, nil))
		}
	}
	return out
}
