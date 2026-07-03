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

func toChecklistItem(c model.AdminTaskChecklistItem) dto.AdminTaskChecklistItem {
	return dto.AdminTaskChecklistItem{
		ID:        c.ID,
		TaskID:    c.TaskID,
		Content:   c.Content,
		Done:      c.Done,
		OrderIdx:  c.OrderIdx,
		CreatedAt: c.CreatedAt,
	}
}

// checklistResponse 组装某任务的子清单与完成计数。
func checklistResponse(db *gorm.DB, taskID uint) (*dto.AdminTaskChecklistResponse, error) {
	var rows []model.AdminTaskChecklistItem
	if err := db.Where("task_id = ?", taskID).Order("order_idx ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询子清单失败: %w", err)
	}
	out := &dto.AdminTaskChecklistResponse{Total: len(rows)}
	for _, r := range rows {
		if r.Done {
			out.Done++
		}
		out.Items = append(out.Items, toChecklistItem(r))
	}
	return out, nil
}

// ListChecklist 返回任务的子清单。
func ListChecklist(ctx context.Context, taskID uint) (*dto.AdminTaskChecklistResponse, error) {
	return checklistResponse(dao.DB.WithContext(ctx), taskID)
}

// AddChecklistItem 给任务新增一条子任务,落在末尾。
func AddChecklistItem(ctx context.Context, taskID uint, content string, actorID uint, actorName string) (*dto.AdminTaskChecklistResponse, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("admin: 子任务内容不能为空")
	}
	db := dao.DB.WithContext(ctx)
	var task model.AdminTask
	if err := db.First(&task, taskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("admin: 任务不存在")
		}
		return nil, fmt.Errorf("admin: 查询任务失败: %w", err)
	}

	var maxIdx *int
	db.Model(&model.AdminTaskChecklistItem{}).Where("task_id = ?", taskID).Select("MAX(order_idx)").Scan(&maxIdx)
	next := 0
	if maxIdx != nil {
		next = *maxIdx + 1
	}
	item := &model.AdminTaskChecklistItem{TaskID: taskID, Content: content, OrderIdx: next}
	if err := db.Create(item).Error; err != nil {
		return nil, fmt.Errorf("admin: 新增子任务失败: %w", err)
	}
	recordActivity(db, taskID, actorID, actorName, "checklist", "新增子任务:"+truncate(content, 40))
	return checklistResponse(db, taskID)
}

// UpdateChecklistItem 更新子任务内容或完成状态。
func UpdateChecklistItem(ctx context.Context, taskID, itemID uint, req dto.AdminTaskChecklistUpdateRequest, actorID uint, actorName string) (*dto.AdminTaskChecklistResponse, error) {
	db := dao.DB.WithContext(ctx)
	var item model.AdminTaskChecklistItem
	if err := db.Where("id = ? AND task_id = ?", itemID, taskID).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("admin: 子任务不存在")
		}
		return nil, fmt.Errorf("admin: 查询子任务失败: %w", err)
	}
	var note string
	if req.Content != nil {
		content := strings.TrimSpace(*req.Content)
		if content == "" {
			return nil, fmt.Errorf("admin: 子任务内容不能为空")
		}
		item.Content = content
	}
	if req.Done != nil && *req.Done != item.Done {
		item.Done = *req.Done
		if item.Done {
			note = "完成子任务:" + truncate(item.Content, 40)
		} else {
			note = "重开子任务:" + truncate(item.Content, 40)
		}
	}
	if err := db.Save(&item).Error; err != nil {
		return nil, fmt.Errorf("admin: 更新子任务失败: %w", err)
	}
	if note != "" {
		recordActivity(db, taskID, actorID, actorName, "checklist", note)
	}
	return checklistResponse(db, taskID)
}

// DeleteChecklistItem 删除一条子任务。
func DeleteChecklistItem(ctx context.Context, taskID, itemID uint) (*dto.AdminTaskChecklistResponse, error) {
	db := dao.DB.WithContext(ctx)
	if err := db.Where("id = ? AND task_id = ?", itemID, taskID).
		Delete(&model.AdminTaskChecklistItem{}).Error; err != nil {
		return nil, fmt.Errorf("admin: 删除子任务失败: %w", err)
	}
	return checklistResponse(db, taskID)
}

// truncate 截断过长文本,便于写入活动明细。
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
