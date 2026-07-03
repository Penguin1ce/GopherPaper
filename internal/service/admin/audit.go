package admin

import (
	"context"
	"fmt"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
	"GopherPaper/internal/zlog"
)

// RecordAudit 异步安全地写入一条管理员操作审计记录,失败仅记日志不影响主流程。
func RecordAudit(ctx context.Context, adminID uint, adminName, method, path, ip string, status int) {
	if dao.DB == nil {
		return
	}
	rec := &model.AdminAuditLog{
		AdminID:   adminID,
		AdminName: adminName,
		Method:    method,
		Path:      path,
		Status:    status,
		IP:        ip,
	}
	if err := dao.DB.WithContext(context.WithoutCancel(ctx)).Create(rec).Error; err != nil {
		zlog.Error("record admin audit failed", "path", path, "err", err)
	}
}

// ListAudit 分页返回管理员操作审计记录,按时间倒序。
func ListAudit(ctx context.Context, page, pageSize int) (*dto.AdminAuditListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	db := dao.DB.WithContext(ctx).Model(&model.AdminAuditLog{})

	out := &dto.AdminAuditListResponse{Page: page, PageSize: pageSize}
	if err := db.Count(&out.Total).Error; err != nil {
		return nil, fmt.Errorf("admin: 统计审计记录失败: %w", err)
	}

	var rows []model.AdminAuditLog
	if err := db.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询审计记录失败: %w", err)
	}

	out.Items = make([]dto.AdminAuditItem, 0, len(rows))
	for _, r := range rows {
		out.Items = append(out.Items, dto.AdminAuditItem{
			ID:        r.ID,
			AdminID:   r.AdminID,
			AdminName: r.AdminName,
			Method:    r.Method,
			Path:      r.Path,
			Status:    r.Status,
			IP:        r.IP,
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}
