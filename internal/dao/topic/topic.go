// Package topic 是会话主题的数据访问层，复用 dao.DB。主题软删。
// 归类引擎在 internal/ai/topic，本包只管增删查改与会话改挂。
package topic

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/model"
)

// ListTopics 列出某学生某 agent 的全部主题，按更新时间倒序。
func ListTopics(ctx context.Context, studentID, agentType string) ([]model.Topic, error) {
	var topics []model.Topic
	err := dao.DB.WithContext(ctx).
		Where("student_id = ? and agent_type = ?", studentID, agentType).
		Order("updated_at desc").
		Find(&topics).Error
	if err != nil {
		return nil, fmt.Errorf("dao/topic: 查询主题列表失败: %w", err)
	}
	return topics, nil
}

// CreateTopic 新建主题，UUID 主键由模型钩子生成。
func CreateTopic(ctx context.Context, t *model.Topic) error {
	if err := dao.DB.WithContext(ctx).Create(t).Error; err != nil {
		return fmt.Errorf("dao/topic: 创建主题失败: %w", err)
	}
	return nil
}

// UpdateTopicCentroid 更新主题质心与成员数。
func UpdateTopicCentroid(ctx context.Context, id, centroid string, memberCount int) error {
	err := dao.DB.WithContext(ctx).
		Model(&model.Topic{}).
		Where("id = ?", id).
		Updates(map[string]any{"centroid": centroid, "member_count": memberCount}).Error
	if err != nil {
		return fmt.Errorf("dao/topic: 更新主题质心失败: %w", err)
	}
	return nil
}

// RenameTopic 重命名主题。
func RenameTopic(ctx context.Context, id, name string) error {
	err := dao.DB.WithContext(ctx).
		Model(&model.Topic{}).
		Where("id = ?", id).
		Update("name", name).Error
	if err != nil {
		return fmt.Errorf("dao/topic: 重命名主题失败: %w", err)
	}
	return nil
}

// MergeTopics 把 src 主题的会话改挂到 dst，更新 dst 质心与成员数后软删 src，整体在事务内完成。
func MergeTopics(ctx context.Context, srcID, dstID, dstCentroid string, dstMemberCount int) error {
	err := dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Session{}).
			Where("topic_id = ?", srcID).
			Update("topic_id", dstID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Topic{}).
			Where("id = ?", dstID).
			Updates(map[string]any{"centroid": dstCentroid, "member_count": dstMemberCount}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", srcID).Delete(&model.Topic{}).Error
	})
	if err != nil {
		return fmt.Errorf("dao/topic: 合并主题失败: %w", err)
	}
	return nil
}

// ListSessionsToClassify 列出某学生某 agent 尚未归类(topic_id 为空)的会话，供存量回填,按创建时间升序。
func ListSessionsToClassify(ctx context.Context, studentID, agentType string) ([]model.Session, error) {
	var sessions []model.Session
	err := dao.DB.WithContext(ctx).
		Where("student_id = ? and agent_type = ? and (topic_id is null or topic_id = '')", studentID, agentType).
		Order("created_at asc").
		Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("dao/topic: 查询未归类会话失败: %w", err)
	}
	return sessions, nil
}

// SetSessionTopic 把会话改挂到某主题，topicID 空表示清除归类。
func SetSessionTopic(ctx context.Context, sessionID, topicID string) error {
	err := dao.DB.WithContext(ctx).
		Model(&model.Session{}).
		Where("id = ?", sessionID).
		Update("topic_id", topicID).Error
	if err != nil {
		return fmt.Errorf("dao/topic: 更新会话主题失败: %w", err)
	}
	return nil
}

// SetSessionTopicVec 同时落会话的主题归属与本次归类向量，供重排时从旧主题精确扣减。
func SetSessionTopicVec(ctx context.Context, sessionID, topicID, vec string) error {
	err := dao.DB.WithContext(ctx).
		Model(&model.Session{}).
		Where("id = ?", sessionID).
		Updates(map[string]any{"topic_id": topicID, "topic_vec": vec}).Error
	if err != nil {
		return fmt.Errorf("dao/topic: 更新会话主题与向量失败: %w", err)
	}
	return nil
}

// DeleteTopic 软删主题，重排把某主题最后一个成员搬走后清理空主题用。
func DeleteTopic(ctx context.Context, id string) error {
	if err := dao.DB.WithContext(ctx).Where("id = ?", id).Delete(&model.Topic{}).Error; err != nil {
		return fmt.Errorf("dao/topic: 删除主题失败: %w", err)
	}
	return nil
}

// ClearTopics 清空某学生某 agent 的全部主题:会话退回未归类(清 topic_id 与 topic_vec)并删除全部主题,
// 事务内完成,返回删除的主题数。供演示重置归类用,之后可经回填重新归类。
func ClearTopics(ctx context.Context, studentID, agentType string) (int64, error) {
	var removed int64
	err := dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Session{}).
			Where("student_id = ? and agent_type = ?", studentID, agentType).
			Updates(map[string]any{"topic_id": "", "topic_vec": ""}).Error; err != nil {
			return err
		}
		res := tx.Where("student_id = ? and agent_type = ?", studentID, agentType).Delete(&model.Topic{})
		if res.Error != nil {
			return res.Error
		}
		removed = res.RowsAffected
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("dao/topic: 清空主题失败: %w", err)
	}
	return removed, nil
}

// CountSessionsForTopic 统计某主题当前的会话数，软删合并后清理空主题用。
func CountSessionsForTopic(ctx context.Context, topicID string) (int64, error) {
	var n int64
	err := dao.DB.WithContext(ctx).
		Model(&model.Session{}).
		Where("topic_id = ?", topicID).
		Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("dao/topic: 统计主题会话数失败: %w", err)
	}
	return n, nil
}
