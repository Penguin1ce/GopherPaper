package admin

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
)

// recordActivity 追加一条任务活动记录。写失败仅忽略(时间线为辅助信息,不阻断主流程)。
func recordActivity(db *gorm.DB, taskID, actorID uint, actorName, action, detail string) {
	if taskID == 0 || action == "" {
		return
	}
	_ = db.Create(&model.AdminTaskActivity{
		TaskID:    taskID,
		ActorID:   actorID,
		ActorName: actorName,
		Action:    action,
		Detail:    detail,
	}).Error
}

// ListTaskActivities 返回任务的活动时间线,按时间倒序,最多 100 条。
func ListTaskActivities(ctx context.Context, taskID uint) (*dto.AdminTaskActivityListResponse, error) {
	db := dao.DB.WithContext(ctx)
	var rows []model.AdminTaskActivity
	if err := db.Where("task_id = ?", taskID).
		Order("created_at DESC").
		Limit(100).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询任务活动失败: %w", err)
	}
	out := &dto.AdminTaskActivityListResponse{Total: int64(len(rows))}
	for _, r := range rows {
		out.Items = append(out.Items, toActivityItem(r))
	}
	return out, nil
}

func toActivityItem(a model.AdminTaskActivity) dto.AdminTaskActivityItem {
	return dto.AdminTaskActivityItem{
		ID:        a.ID,
		TaskID:    a.TaskID,
		ActorName: a.ActorName,
		Action:    a.Action,
		Detail:    a.Detail,
		CreatedAt: a.CreatedAt,
	}
}

// GetTaskDetail 聚合任务详情抽屉所需的全部数据:任务本体、子清单、评论、活动时间线。
func GetTaskDetail(ctx context.Context, id uint) (*dto.AdminTaskDetail, error) {
	db := dao.DB.WithContext(ctx)

	var t model.AdminTask
	if err := db.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("admin: 任务不存在")
		}
		return nil, fmt.Errorf("admin: 查询任务失败: %w", err)
	}

	out := &dto.AdminTaskDetail{}

	// 评论。
	var comments []model.AdminTaskComment
	db.Where("task_id = ?", id).Order("created_at ASC").Find(&comments)
	t.CommentCount = len(comments)
	out.Task = toTaskItem(t)
	for _, c := range comments {
		out.Comments = append(out.Comments, dto.AdminTaskComment{
			ID:         c.ID,
			TaskID:     c.TaskID,
			AuthorName: c.AuthorName,
			Content:    c.Content,
			CreatedAt:  c.CreatedAt,
		})
	}

	// 子清单。
	var checks []model.AdminTaskChecklistItem
	db.Where("task_id = ?", id).Order("order_idx ASC, id ASC").Find(&checks)
	doneCount := 0
	for _, c := range checks {
		if c.Done {
			doneCount++
		}
		out.Checklist.Items = append(out.Checklist.Items, toChecklistItem(c))
	}
	out.Checklist.Total = len(checks)
	out.Checklist.Done = doneCount

	// 活动时间线。
	var acts []model.AdminTaskActivity
	db.Where("task_id = ?", id).Order("created_at DESC").Limit(100).Find(&acts)
	for _, a := range acts {
		out.Activities = append(out.Activities, toActivityItem(a))
	}

	return out, nil
}
