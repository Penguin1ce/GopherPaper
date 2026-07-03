package admin

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
)

// BulkTasks 对一组任务执行批量操作:move(改列)/assign(改负责人)/priority(改优先级)/delete(删除)。
// 逐条处理,单条失败不影响其余,返回成功/失败统计。
func BulkTasks(ctx context.Context, req dto.AdminTaskBulkRequest, actorID uint, actorName string) (*dto.AdminBatchResult, error) {
	action := strings.TrimSpace(req.Action)
	res := &dto.AdminBatchResult{Requested: len(req.IDs)}

	seen := make(map[uint]struct{}, len(req.IDs))
	db := dao.DB.WithContext(ctx)

	for _, id := range req.IDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		var err error
		switch action {
		case "move":
			status := normalizeTaskStatus(req.Status)
			err = db.Model(&model.AdminTask{}).Where("id = ?", id).
				Updates(map[string]any{"status": status, "order_idx": nextOrderIdx(db, status)}).Error
			if err == nil {
				recordActivity(db, id, actorID, actorName, "moved", "批量移动到 "+statusLabel(status))
			}
		case "assign":
			assignee := strings.TrimSpace(req.Assignee)
			err = db.Model(&model.AdminTask{}).Where("id = ?", id).Update("assignee", assignee).Error
			if err == nil {
				recordActivity(db, id, actorID, actorName, "assigned", "批量指派给 "+fallback(assignee, "(未指派)"))
			}
		case "priority":
			priority := normalizeTaskPriority(req.Priority)
			err = db.Model(&model.AdminTask{}).Where("id = ?", id).Update("priority", priority).Error
			if err == nil {
				recordActivity(db, id, actorID, actorName, "edited", "批量调整优先级为 "+priority)
			}
		case "delete":
			err = deleteTaskTx(db, id)
		default:
			return nil, fmt.Errorf("admin: 不支持的批量操作 %q", action)
		}

		if err != nil {
			res.Failed++
		} else {
			res.Succeeded++
		}
	}
	return res, nil
}

// deleteTaskTx 在事务里删除任务及其从属数据。
func deleteTaskTx(db *gorm.DB, id uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ?", id).Delete(&model.AdminTaskComment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_id = ?", id).Delete(&model.AdminTaskChecklistItem{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_id = ?", id).Delete(&model.AdminTaskActivity{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.AdminTask{}, id).Error
	})
}

func fallback(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// ExportTasks 返回用于 CSV 导出的任务行(最多 exportLimit 条)。
func ExportTasks(ctx context.Context) ([]model.AdminTask, error) {
	var rows []model.AdminTask
	if err := dao.DB.WithContext(ctx).
		Order("updated_at DESC").
		Limit(exportLimit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 导出任务失败: %w", err)
	}
	return rows, nil
}
