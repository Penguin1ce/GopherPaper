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

// ListFeedbacks 分页返回用户反馈工单,可按状态筛选,按创建时间倒序。
func ListFeedbacks(ctx context.Context, status string, page, pageSize int) (*dto.AdminFeedbackListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	db := dao.DB.WithContext(ctx).Model(&model.Feedback{})
	if s := strings.TrimSpace(status); s != "" {
		db = db.Where("status = ?", s)
	}

	out := &dto.AdminFeedbackListResponse{Page: page, PageSize: pageSize}
	if err := db.Count(&out.Total).Error; err != nil {
		return nil, fmt.Errorf("admin: 统计反馈失败: %w", err)
	}

	var rows []model.Feedback
	if err := db.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询反馈失败: %w", err)
	}
	for _, r := range rows {
		out.Items = append(out.Items, dto.AdminFeedbackItem{
			ID:          r.ID,
			StudentID:   r.StudentID,
			Category:    r.Category,
			Content:     r.Content,
			Status:      r.Status,
			Reply:       r.Reply,
			HandlerName: r.HandlerName,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
		})
	}
	return out, nil
}

// UpdateFeedback 处理反馈:更新状态与回复,记录处理人。
func UpdateFeedback(ctx context.Context, id uint, req dto.AdminFeedbackUpdateRequest, handlerName string) error {
	db := dao.DB.WithContext(ctx)
	var f model.Feedback
	if err := db.First(&f, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("admin: 反馈不存在")
		}
		return fmt.Errorf("admin: 查询反馈失败: %w", err)
	}
	updates := map[string]any{}
	if s := strings.TrimSpace(req.Status); s != "" {
		updates["status"] = s
	}
	updates["reply"] = req.Reply
	updates["handler_name"] = handlerName
	if err := db.Model(&f).Updates(updates).Error; err != nil {
		return fmt.Errorf("admin: 更新反馈失败: %w", err)
	}
	return nil
}

// SeedDemoFeedbacks 造几条演示反馈,便于展示反馈面板。
func SeedDemoFeedbacks(ctx context.Context) (int, error) {
	db := dao.DB.WithContext(ctx)
	samples := []model.Feedback{
		{StudentID: "demo-0001", Category: "bug", Content: "上传大 PDF 偶尔解析卡在 90%。", Status: "open"},
		{StudentID: "demo-0003", Category: "feature", Content: "希望研读报告支持导出 Word。", Status: "open"},
		{StudentID: "demo-0005", Category: "other", Content: "小云雀检索速度很快,点赞!", Status: "resolved", Reply: "感谢反馈~", HandlerName: "root"},
	}
	n := 0
	for _, s := range samples {
		if err := db.Create(&s).Error; err != nil {
			return n, fmt.Errorf("admin: 写入演示反馈失败: %w", err)
		}
		n++
	}
	return n, nil
}
