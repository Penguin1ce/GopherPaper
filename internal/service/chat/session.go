package chat

import (
	"context"

	chatdao "GopherPaper/internal/dao/chat"
	"GopherPaper/internal/history"
	"GopherPaper/internal/model"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/errs"
)

// CreateSession 为用户新建一段会话，paperID 可空表示跨库问答。
func CreateSession(ctx context.Context, studentID, paperID, title string) (*model.Session, error) {
	s := &model.Session{StudentID: studentID, PaperID: paperID, Title: title}
	if err := chatdao.CreateSession(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

// ListSessions 列出学生的全部会话。
func ListSessions(ctx context.Context, studentID string) ([]model.Session, error) {
	return chatdao.ListSessions(ctx, studentID)
}

// DeleteSession 删除会话，仅限本人。会话元数据软删，历史事件从 Session 清理。
func DeleteSession(ctx context.Context, studentID, sessionID string) error {
	if _, err := ownedSession(ctx, studentID, sessionID); err != nil {
		return err
	}
	if err := chatdao.DeleteSession(ctx, sessionID); err != nil {
		return err
	}
	if err := history.Delete(ctx, studentID, sessionID); err != nil {
		zlog.Error("清理会话历史失败", "session_id", sessionID, "err", err)
	}
	return nil
}

// ListMessages 还原会话历史消息，仅限本人，从 trpc Session 读。
func ListMessages(ctx context.Context, studentID, sessionID string) ([]model.Message, error) {
	if _, err := ownedSession(ctx, studentID, sessionID); err != nil {
		return nil, err
	}
	return history.List(ctx, studentID, sessionID)
}

// ownedSession 取会话并校验归属，非本人返回 errs.ErrSessionForbidden。
func ownedSession(ctx context.Context, studentID, sessionID string) (*model.Session, error) {
	s, err := chatdao.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if s.StudentID != studentID {
		return nil, errs.ErrSessionForbidden
	}
	return s, nil
}
