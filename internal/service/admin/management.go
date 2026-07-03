package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
)

// ── 标签管理 ─────────────────────────────────────────────────

// ListTags 返回全部标签及各自被引用的论文数。
func ListTags(ctx context.Context) (*dto.AdminTagListResponse, error) {
	db := dao.DB.WithContext(ctx)
	var tags []model.Tag
	if err := db.Order("owner_id, name").Find(&tags).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询标签失败: %w", err)
	}

	type tc struct {
		TagID uint64
		Total int64
	}
	var counts []tc
	db.Model(&model.PaperTag{}).
		Select("tag_id, COUNT(*) as total").
		Group("tag_id").
		Scan(&counts)
	countMap := make(map[uint64]int64, len(counts))
	for _, c := range counts {
		countMap[c.TagID] = c.Total
	}

	out := &dto.AdminTagListResponse{Total: int64(len(tags))}
	for _, t := range tags {
		out.Items = append(out.Items, dto.AdminTagItem{
			ID:         t.ID,
			OwnerID:    t.OwnerID,
			Name:       t.Name,
			PaperCount: countMap[t.ID],
		})
	}
	return out, nil
}

// RenameTag 重命名标签。
func RenameTag(ctx context.Context, id uint64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("admin: 标签名不能为空")
	}
	if err := dao.DB.WithContext(ctx).Model(&model.Tag{}).
		Where("id = ?", id).
		Update("name", name).Error; err != nil {
		return fmt.Errorf("admin: 重命名标签失败: %w", err)
	}
	return nil
}

// DeleteTag 删除标签及其所有论文关联。
func DeleteTag(ctx context.Context, id uint64) error {
	return dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tag_id = ?", id).Delete(&model.PaperTag{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Tag{}, id).Error
	})
}

// MergeTags 把 source 标签合并进 target:论文关联迁移到 target,删除 source。
func MergeTags(ctx context.Context, sourceID, targetID uint64) error {
	if sourceID == targetID {
		return fmt.Errorf("admin: 源标签与目标标签相同")
	}
	return dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var links []model.PaperTag
		if err := tx.Where("tag_id = ?", sourceID).Find(&links).Error; err != nil {
			return err
		}
		for _, l := range links {
			var cnt int64
			tx.Model(&model.PaperTag{}).Where("paper_id = ? AND tag_id = ?", l.PaperID, targetID).Count(&cnt)
			if cnt == 0 {
				if err := tx.Create(&model.PaperTag{PaperID: l.PaperID, TagID: targetID}).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Where("tag_id = ?", sourceID).Delete(&model.PaperTag{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Tag{}, sourceID).Error
	})
}

// ── 会话管理 ─────────────────────────────────────────────────

// ListSessions 分页返回会话,可按 agent_type 筛选,按创建时间倒序。
func ListSessions(ctx context.Context, agentType string, page, pageSize int) (*dto.AdminSessionListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 15
	}
	db := dao.DB.WithContext(ctx).Model(&model.Session{})
	if a := strings.TrimSpace(agentType); a != "" {
		db = db.Where("agent_type = ?", a)
	}

	out := &dto.AdminSessionListResponse{Page: page, PageSize: pageSize}
	if err := db.Count(&out.Total).Error; err != nil {
		return nil, fmt.Errorf("admin: 统计会话失败: %w", err)
	}

	var rows []model.Session
	if err := db.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询会话失败: %w", err)
	}
	for _, r := range rows {
		out.Items = append(out.Items, dto.AdminSessionItem{
			ID:        r.ID,
			StudentID: r.StudentID,
			PaperID:   r.PaperID,
			AgentType: r.AgentType,
			Title:     r.Title,
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

// DeleteSession 软删除一条会话。
func DeleteSession(ctx context.Context, id string) error {
	if err := dao.DB.WithContext(ctx).Where("id = ?", id).Delete(&model.Session{}).Error; err != nil {
		return fmt.Errorf("admin: 删除会话失败: %w", err)
	}
	return nil
}

// ── 管理员账号管理 ───────────────────────────────────────────

// ListAdmins 返回全部管理员账号。
func ListAdmins(ctx context.Context) (*dto.AdminAccountListResponse, error) {
	var rows []model.Admin
	if err := dao.DB.WithContext(ctx).Order("created_at").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询管理员失败: %w", err)
	}
	out := &dto.AdminAccountListResponse{}
	for _, r := range rows {
		out.Items = append(out.Items, dto.AdminAccountItem{
			ID:          r.ID,
			Username:    r.Username,
			Email:       r.Email,
			Name:        r.Name,
			Status:      r.Status,
			LastLoginAt: r.LastLoginAt,
			CreatedAt:   r.CreatedAt,
		})
	}
	return out, nil
}

// SetAdminStatus 启用/停用管理员账号。
func SetAdminStatus(ctx context.Context, id uint, status string) error {
	status = strings.TrimSpace(status)
	if status != "active" && status != "disabled" {
		return fmt.Errorf("admin: 非法状态")
	}
	db := dao.DB.WithContext(ctx)
	var admin model.Admin
	if err := db.First(&admin, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("admin: 管理员不存在")
		}
		return fmt.Errorf("admin: 查询管理员失败: %w", err)
	}
	if err := db.Model(&admin).Update("status", status).Error; err != nil {
		return fmt.Errorf("admin: 更新管理员状态失败: %w", err)
	}
	return nil
}
