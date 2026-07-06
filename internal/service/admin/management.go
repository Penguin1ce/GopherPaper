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
