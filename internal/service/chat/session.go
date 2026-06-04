package chat

import (
	"context"

	chatdao "GopherPaper/internal/dao/chat"
	"GopherPaper/internal/model"
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

// DeleteSession 删除会话，仅限本人。
func DeleteSession(ctx context.Context, studentID, sessionID string) error {
	if _, err := ownedSession(ctx, studentID, sessionID); err != nil {
		return err
	}
	return chatdao.DeleteSession(ctx, sessionID)
}

// ListMessages 还原会话历史消息，仅限本人。
func ListMessages(ctx context.Context, studentID, sessionID string) ([]model.Message, error) {
	if _, err := ownedSession(ctx, studentID, sessionID); err != nil {
		return nil, err
	}
	return chatdao.ListMessages(ctx, sessionID)
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
