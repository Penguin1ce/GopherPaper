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

// ListAnnouncements 分页返回站内公告,按创建时间倒序。
func ListAnnouncements(ctx context.Context, page, pageSize int) (*dto.AdminAnnouncementListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	db := dao.DB.WithContext(ctx).Model(&model.Announcement{})

	out := &dto.AdminAnnouncementListResponse{Page: page, PageSize: pageSize}
	if err := db.Count(&out.Total).Error; err != nil {
		return nil, fmt.Errorf("admin: 统计公告失败: %w", err)
	}

	var rows []model.Announcement
	if err := db.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询公告失败: %w", err)
	}
	for _, r := range rows {
		out.Items = append(out.Items, toAnnouncementItem(r))
	}
	return out, nil
}

// CreateAnnouncement 新建一条公告。
func CreateAnnouncement(ctx context.Context, req dto.AdminAnnouncementRequest, authorID uint, authorName string) (*dto.AdminAnnouncementItem, error) {
	level := normalizeLevel(req.Level)
	a := &model.Announcement{
		Title:      strings.TrimSpace(req.Title),
		Content:    req.Content,
		Level:      level,
		Published:  req.Published,
		AuthorID:   authorID,
		AuthorName: authorName,
	}
	if err := dao.DB.WithContext(ctx).Create(a).Error; err != nil {
		return nil, fmt.Errorf("admin: 创建公告失败: %w", err)
	}
	item := toAnnouncementItem(*a)
	return &item, nil
}

// UpdateAnnouncement 更新公告内容与发布状态。
func UpdateAnnouncement(ctx context.Context, id uint, req dto.AdminAnnouncementRequest) (*dto.AdminAnnouncementItem, error) {
	db := dao.DB.WithContext(ctx)
	var a model.Announcement
	if err := db.First(&a, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("admin: 公告不存在")
		}
		return nil, fmt.Errorf("admin: 查询公告失败: %w", err)
	}
	a.Title = strings.TrimSpace(req.Title)
	a.Content = req.Content
	a.Level = normalizeLevel(req.Level)
	a.Published = req.Published
	if err := db.Save(&a).Error; err != nil {
		return nil, fmt.Errorf("admin: 更新公告失败: %w", err)
	}
	item := toAnnouncementItem(a)
	return &item, nil
}

// DeleteAnnouncement 软删除一条公告。
func DeleteAnnouncement(ctx context.Context, id uint) error {
	if err := dao.DB.WithContext(ctx).Delete(&model.Announcement{}, id).Error; err != nil {
		return fmt.Errorf("admin: 删除公告失败: %w", err)
	}
	return nil
}

func toAnnouncementItem(a model.Announcement) dto.AdminAnnouncementItem {
	return dto.AdminAnnouncementItem{
		ID:         a.ID,
		Title:      a.Title,
		Content:    a.Content,
		Level:      a.Level,
		Published:  a.Published,
		AuthorName: a.AuthorName,
		CreatedAt:  a.CreatedAt,
		UpdatedAt:  a.UpdatedAt,
	}
}

func normalizeLevel(level string) string {
	switch strings.TrimSpace(level) {
	case "warning", "critical":
		return level
	default:
		return "info"
	}
}
