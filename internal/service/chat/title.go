package chat

import (
	"context"
	"strings"

	"GopherPaper/internal/ai"
	chatdao "GopherPaper/internal/dao/chat"
	"GopherPaper/internal/model"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

func shouldRewriteSessionTitle(sess *model.Session, hist []model.Message) bool {
	if sess == nil || strings.TrimSpace(sess.PaperID) == "" || strings.TrimSpace(sess.AgentType) != "" {
		return false
	}
	for _, msg := range hist {
		if msg.Role == model.RoleUser && strings.TrimSpace(msg.Content) != "" {
			return false
		}
	}
	return true
}

func rewriteAndSaveSessionTitle(ctx context.Context, studentID, sessionID, firstQuestion string) {
	firstQuestion = strings.TrimSpace(firstQuestion)
	if firstQuestion == "" {
		return
	}

	titleCtx, cancel := context.WithTimeout(ctx, constant.SessionTitleRewriteTimeout)
	title, err := ai.RewriteSessionTitle(titleCtx, firstQuestion)
	cancel()
	if err != nil {
		zlog.Warn("会话标题改写失败,回退首问片段", "student_id", studentID, "session_id", sessionID, "err", err)
		title = ai.FallbackSessionTitle(firstQuestion)
	}
	title = truncateTitle(strings.TrimSpace(title))
	if title == "" {
		return
	}
	if err := chatdao.UpdateSessionTitle(context.WithoutCancel(ctx), sessionID, title); err != nil {
		zlog.Warn("更新会话标题失败", "student_id", studentID, "session_id", sessionID, "err", err)
		return
	}
	zlog.Info("已根据首问更新会话标题", "student_id", studentID, "session_id", sessionID, "title", title)
}
