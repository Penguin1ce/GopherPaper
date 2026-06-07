// Package chat 是会话元数据的数据访问层，复用 dao.DB。会话软删。
// 消息历史由 internal/history 管理。
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

// DeleteSession 软删会话。消息历史由调用方另行处理。
func DeleteSession(ctx context.Context, id string) error {
	return dao.DB.WithContext(ctx).Where("id = ?", id).Delete(&model.Session{}).Error
}

// TouchSession 刷新会话 updated_at，让有新消息的会话排到列表前面。
func TouchSession(ctx context.Context, id string) error {
	return dao.DB.WithContext(ctx).
		Model(&model.Session{}).
		Where("id = ?", id).
		Update("updated_at", time.Now()).Error
}
