// Package chat 是会话与消息的数据访问层，复用 dao.DB。
// 会话软删、消息追加写不可变，多轮上下文按 created_at 升序还原。
package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/errs"
)

// CreateSession 新建会话，UUID 主键由模型钩子生成。
func CreateSession(ctx context.Context, s *model.Session) error {
	if err := dao.DB.WithContext(ctx).Create(s).Error; err != nil {
		return fmt.Errorf("dao/chat: 创建会话失败: %w", err)
	}
	return nil
}

// ListSessions 按更新时间倒序列出某学生的全部会话。
func ListSessions(ctx context.Context, studentID string) ([]model.Session, error) {
	var sessions []model.Session
	err := dao.DB.WithContext(ctx).
		Where("student_id = ?", studentID).
		Order("updated_at desc").
		Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("dao/chat: 查询会话列表失败: %w", err)
	}
	return sessions, nil
}

// GetSession 按 id 取会话，不存在返回 errs.ErrSessionNotFound。
func GetSession(ctx context.Context, id string) (*model.Session, error) {
	var s model.Session
	err := dao.DB.WithContext(ctx).Where("id = ?", id).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("dao/chat: 查询会话失败: %w", err)
	}
	return &s, nil
}

// DeleteSession 软删会话并硬删其消息，事务保证一致。
func DeleteSession(ctx context.Context, id string) error {
	return dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", id).Delete(&model.Message{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&model.Session{}).Error
	})
}

// CreateMessages 批量追加消息，一次落库用户与助教两条。
func CreateMessages(ctx context.Context, msgs []*model.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	if err := dao.DB.WithContext(ctx).Create(msgs).Error; err != nil {
		return fmt.Errorf("dao/chat: 写入消息失败: %w", err)
	}
	return nil
}

// ListMessages 按时间升序还原会话全部消息，供前端展示历史。
func ListMessages(ctx context.Context, sessionID string) ([]model.Message, error) {
	var msgs []model.Message
	err := dao.DB.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at asc, id asc").
		Find(&msgs).Error
	if err != nil {
		return nil, fmt.Errorf("dao/chat: 查询消息失败: %w", err)
	}
	return msgs, nil
}

// RecentMessages 取最近 limit 条并按时间升序返回，用于拼多轮上下文。limit<=0 取全部。
func RecentMessages(ctx context.Context, sessionID string, limit int) ([]model.Message, error) {
	var msgs []model.Message
	q := dao.DB.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at desc, id desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&msgs).Error; err != nil {
		return nil, fmt.Errorf("dao/chat: 查询最近消息失败: %w", err)
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// TouchSession 刷新会话 updated_at，让有新消息的会话排到列表前面。
func TouchSession(ctx context.Context, id string) error {
	return dao.DB.WithContext(ctx).
		Model(&model.Session{}).
		Where("id = ?", id).
		Update("updated_at", time.Now()).Error
}
