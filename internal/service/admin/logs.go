package admin

import (
	"context"
	"fmt"
	"strings"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
)

// ListLogs 分页返回服务调用日志,支持按服务类型、成功/失败、调用者筛选,按时间倒序。
func ListLogs(ctx context.Context, serviceType, result, actor string, page, pageSize int) (*dto.AdminLogListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	db := dao.DB.WithContext(ctx).Model(&model.ServiceCallLog{})

	if st := strings.TrimSpace(serviceType); st != "" {
		db = db.Where("service_type = ?", st)
	}
	switch strings.TrimSpace(result) {
	case "success":
		db = db.Where("success = ?", true)
	case "failed":
		db = db.Where("success = ?", false)
	}
	if a := strings.TrimSpace(actor); a != "" {
		db = db.Where("actor_id LIKE ?", "%"+a+"%")
	}

	out := &dto.AdminLogListResponse{Page: page, PageSize: pageSize}
	if err := db.Count(&out.Total).Error; err != nil {
		return nil, fmt.Errorf("admin: 统计日志失败: %w", err)
	}

	var logs []model.ServiceCallLog
	if err := db.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询日志失败: %w", err)
	}

	out.Items = make([]dto.AdminLogItem, 0, len(logs))
	for _, l := range logs {
		out.Items = append(out.Items, dto.AdminLogItem{
			ID:           l.ID,
			ServiceType:  l.ServiceType,
			ActorID:      l.ActorID,
			PaperID:      l.PaperID,
			SessionID:    l.SessionID,
			Success:      l.Success,
			DurationMS:   l.DurationMS,
			ErrorMessage: l.ErrorMessage,
			CreatedAt:    l.CreatedAt,
		})
	}
	return out, nil
}

// LogStats 返回日志筛选面板需要的服务类型枚举与总体统计。
func LogStats(ctx context.Context) (*dto.AdminLogStats, error) {
	db := dao.DB.WithContext(ctx)
	out := &dto.AdminLogStats{}

	if err := db.Model(&model.ServiceCallLog{}).
		Distinct().
		Order("service_type").
		Pluck("service_type", &out.ServiceTypes).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询服务类型失败: %w", err)
	}

	db.Model(&model.ServiceCallLog{}).Count(&out.Total)
	db.Model(&model.ServiceCallLog{}).Where("success = ?", true).Count(&out.Success)
	out.Failed = out.Total - out.Success

	type agg struct {
		AvgMS float64
		MaxMS int64
	}
	var a agg
	db.Model(&model.ServiceCallLog{}).
		Select("AVG(duration_ms) as avg_ms, MAX(duration_ms) as max_ms").
		Scan(&a)
	out.AvgMS = a.AvgMS
	out.MaxMS = a.MaxMS

	return out, nil
}
