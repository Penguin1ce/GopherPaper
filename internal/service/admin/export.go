package admin

import (
	"context"
	"fmt"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/model"
)

// 导出上限,防止一次拉取过多数据拖垮内存。
const exportLimit = 5000

// ExportPapers 返回用于导出的论文行(最多 exportLimit 条)。
func ExportPapers(ctx context.Context) ([]model.Paper, error) {
	var rows []model.Paper
	if err := dao.DB.WithContext(ctx).
		Order("created_at DESC").
		Limit(exportLimit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 导出论文失败: %w", err)
	}
	return rows, nil
}

// ExportUsers 返回用于导出的用户行。
func ExportUsers(ctx context.Context) ([]model.User, error) {
	var rows []model.User
	if err := dao.DB.WithContext(ctx).
		Order("created_at DESC").
		Limit(exportLimit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 导出用户失败: %w", err)
	}
	return rows, nil
}

// ExportLogs 返回用于导出的服务调用日志行。
func ExportLogs(ctx context.Context) ([]model.ServiceCallLog, error) {
	var rows []model.ServiceCallLog
	if err := dao.DB.WithContext(ctx).
		Order("created_at DESC").
		Limit(exportLimit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 导出日志失败: %w", err)
	}
	return rows, nil
}
