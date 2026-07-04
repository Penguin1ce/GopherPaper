package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
)

// 看板列的顺序与中文标签。
var taskStatusOrder = []struct {
	Status string
	Label  string
}{
	{"todo", "待办"},
	{"doing", "进行中"},
	{"review", "待验收"},
	{"done", "已完成"},
}

var validTaskStatus = map[string]bool{"todo": true, "doing": true, "review": true, "done": true}
var validTaskPriority = map[string]bool{"low": true, "medium": true, "high": true, "urgent": true}

func normalizeTaskStatus(s string) string {
	s = strings.TrimSpace(s)
	if validTaskStatus[s] {
		return s
	}
	return "todo"
}

func normalizeTaskPriority(p string) string {
	p = strings.TrimSpace(p)
	if validTaskPriority[p] {
		return p
	}
	return "medium"
}

func statusLabel(status string) string {
	for _, s := range taskStatusOrder {
		if s.Status == status {
			return s.Label
		}
	}
	return status
}

// cleanLabels 去重、去空白,最多保留 12 个标签。
func cleanLabels(in []string) model.JSONStrings {
	seen := make(map[string]struct{}, len(in))
	out := make(model.JSONStrings, 0, len(in))
	for _, raw := range in {
		l := strings.TrimSpace(raw)
		if l == "" {
			continue
		}
		if _, ok := seen[l]; ok {
			continue
		}
		seen[l] = struct{}{}
		out = append(out, l)
		if len(out) >= 12 {
			break
		}
	}
	return out
}

// toTaskItem 把模型转换为对外视图,并计算是否逾期。
func toTaskItem(t model.AdminTask) dto.AdminTaskItem {
	overdue := t.DueAt != nil && t.Status != "done" && t.DueAt.Before(time.Now())
	return dto.AdminTaskItem{
		ID:           t.ID,
		Title:        t.Title,
		Description:  t.Description,
		Status:       t.Status,
		Priority:     t.Priority,
		Assignee:     t.Assignee,
		Labels:       []string(t.Labels),
		DueAt:        t.DueAt,
		OrderIdx:     t.OrderIdx,
		CreatorName:  t.CreatorName,
		CommentCount: t.CommentCount,
		Overdue:      overdue,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
	}
}

// fillCommentCounts 批量回填一组任务的评论数,避免 N+1 查询。
func fillCommentCounts(db *gorm.DB, tasks []model.AdminTask) {
	if len(tasks) == 0 {
		return
	}
	ids := make([]uint, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	type cc struct {
		TaskID uint
		Total  int64
	}
	var rows []cc
	db.Model(&model.AdminTaskComment{}).
		Select("task_id, COUNT(*) as total").
		Where("task_id IN ?", ids).
		Group("task_id").
		Scan(&rows)
	m := make(map[uint]int, len(rows))
	for _, r := range rows {
		m[r.TaskID] = int(r.Total)
	}
	for i := range tasks {
		tasks[i].CommentCount = m[tasks[i].ID]
	}
}

// BoardTasks 返回按状态分组的看板视图,可按负责人/优先级/关键词筛选。
func BoardTasks(ctx context.Context, assignee, priority, query string) (*dto.AdminTaskBoardResponse, error) {
	db := dao.DB.WithContext(ctx).Model(&model.AdminTask{})
	if a := strings.TrimSpace(assignee); a != "" {
		db = db.Where("assignee = ?", a)
	}
	if p := strings.TrimSpace(priority); p != "" && validTaskPriority[p] {
		db = db.Where("priority = ?", p)
	}
	if q := strings.TrimSpace(query); q != "" {
		like := "%" + q + "%"
		db = db.Where("title LIKE ? OR description LIKE ?", like, like)
	}

	var rows []model.AdminTask
	if err := db.Order("order_idx ASC, updated_at DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询任务看板失败: %w", err)
	}
	fillCommentCounts(dao.DB.WithContext(ctx), rows)

	grouped := make(map[string][]dto.AdminTaskItem)
	for _, r := range rows {
		grouped[r.Status] = append(grouped[r.Status], toTaskItem(r))
	}

	out := &dto.AdminTaskBoardResponse{Total: len(rows)}
	for _, s := range taskStatusOrder {
		items := grouped[s.Status]
		if items == nil {
			items = make([]dto.AdminTaskItem, 0)
		}
		out.Columns = append(out.Columns, dto.AdminTaskColumn{
			Status: s.Status,
			Label:  s.Label,
			Count:  len(items),
			Items:  items,
		})
	}
	return out, nil
}

// ListTasks 分页返回任务列表(表格视图),支持状态/负责人/优先级/关键词筛选。
func ListTasks(ctx context.Context, status, assignee, priority, query string, page, pageSize int) (*dto.AdminTaskListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	db := dao.DB.WithContext(ctx).Model(&model.AdminTask{})
	if s := strings.TrimSpace(status); s != "" && validTaskStatus[s] {
		db = db.Where("status = ?", s)
	}
	if a := strings.TrimSpace(assignee); a != "" {
		db = db.Where("assignee = ?", a)
	}
	if p := strings.TrimSpace(priority); p != "" && validTaskPriority[p] {
		db = db.Where("priority = ?", p)
	}
	if q := strings.TrimSpace(query); q != "" {
		like := "%" + q + "%"
		db = db.Where("title LIKE ? OR description LIKE ?", like, like)
	}

	out := &dto.AdminTaskListResponse{Page: page, PageSize: pageSize}
	if err := db.Count(&out.Total).Error; err != nil {
		return nil, fmt.Errorf("admin: 统计任务失败: %w", err)
	}

	var rows []model.AdminTask
	if err := db.Order("updated_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询任务失败: %w", err)
	}
	fillCommentCounts(dao.DB.WithContext(ctx), rows)
	for _, r := range rows {
		out.Items = append(out.Items, toTaskItem(r))
	}
	return out, nil
}

// nextOrderIdx 返回某状态列末尾的排序值(现有最大值 +1)。
func nextOrderIdx(db *gorm.DB, status string) int {
	var max *int
	db.Model(&model.AdminTask{}).
		Where("status = ?", status).
		Select("MAX(order_idx)").
		Scan(&max)
	if max == nil {
		return 0
	}
	return *max + 1
}

// CreateTask 新建一张任务卡片,默认落在目标列末尾。
func CreateTask(ctx context.Context, req dto.AdminTaskCreateRequest, creatorID uint, creatorName string) (*dto.AdminTaskItem, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, fmt.Errorf("admin: 任务标题不能为空")
	}
	db := dao.DB.WithContext(ctx)
	status := normalizeTaskStatus(req.Status)
	t := &model.AdminTask{
		Title:       title,
		Description: req.Description,
		Status:      status,
		Priority:    normalizeTaskPriority(req.Priority),
		Assignee:    strings.TrimSpace(req.Assignee),
		Labels:      cleanLabels(req.Labels),
		DueAt:       req.DueAt,
		OrderIdx:    nextOrderIdx(db, status),
		CreatorID:   creatorID,
		CreatorName: creatorName,
	}
	if err := db.Create(t).Error; err != nil {
		return nil, fmt.Errorf("admin: 创建任务失败: %w", err)
	}
	recordActivity(db, t.ID, creatorID, creatorName, "created", fmt.Sprintf("创建任务「%s」", title))
	item := toTaskItem(*t)
	return &item, nil
}

// GetTask 读取单张任务(含评论数)。
func GetTask(ctx context.Context, id uint) (*dto.AdminTaskItem, error) {
	db := dao.DB.WithContext(ctx)
	var t model.AdminTask
	if err := db.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("admin: 任务不存在")
		}
		return nil, fmt.Errorf("admin: 查询任务失败: %w", err)
	}
	var cnt int64
	db.Model(&model.AdminTaskComment{}).Where("task_id = ?", id).Count(&cnt)
	t.CommentCount = int(cnt)
	item := toTaskItem(t)
	return &item, nil
}

// UpdateTask 局部更新任务字段,只改传入的非空指针字段,并记录变更明细。
func UpdateTask(ctx context.Context, id uint, req dto.AdminTaskUpdateRequest, actorID uint, actorName string) (*dto.AdminTaskItem, error) {
	db := dao.DB.WithContext(ctx)
	var t model.AdminTask
	if err := db.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("admin: 任务不存在")
		}
		return nil, fmt.Errorf("admin: 查询任务失败: %w", err)
	}

	var changes []string
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, fmt.Errorf("admin: 任务标题不能为空")
		}
		if title != t.Title {
			changes = append(changes, "标题")
		}
		t.Title = title
	}
	if req.Description != nil && *req.Description != t.Description {
		t.Description = *req.Description
		changes = append(changes, "描述")
	}
	if req.Status != nil {
		ns := normalizeTaskStatus(*req.Status)
		if ns != t.Status {
			changes = append(changes, fmt.Sprintf("状态→%s", statusLabel(ns)))
		}
		t.Status = ns
	}
	if req.Priority != nil {
		np := normalizeTaskPriority(*req.Priority)
		if np != t.Priority {
			changes = append(changes, "优先级")
		}
		t.Priority = np
	}
	if req.Assignee != nil {
		na := strings.TrimSpace(*req.Assignee)
		if na != t.Assignee {
			changes = append(changes, "负责人")
		}
		t.Assignee = na
	}
	if req.Labels != nil {
		t.Labels = cleanLabels(*req.Labels)
		changes = append(changes, "标签")
	}
	if req.ClearDue {
		if t.DueAt != nil {
			changes = append(changes, "清除截止时间")
		}
		t.DueAt = nil
	} else if req.DueAt != nil {
		t.DueAt = req.DueAt
		changes = append(changes, "截止时间")
	}
	if err := db.Save(&t).Error; err != nil {
		return nil, fmt.Errorf("admin: 更新任务失败: %w", err)
	}
	if len(changes) > 0 {
		recordActivity(db, t.ID, actorID, actorName, "edited", "修改了 "+strings.Join(changes, "、"))
	}
	var cnt int64
	db.Model(&model.AdminTaskComment{}).Where("task_id = ?", id).Count(&cnt)
	t.CommentCount = int(cnt)
	item := toTaskItem(t)
	return &item, nil
}

// MoveTask 把卡片移动到目标列,默认置于该列末尾,并记录活动。
func MoveTask(ctx context.Context, id uint, req dto.AdminTaskMoveRequest, actorID uint, actorName string) (*dto.AdminTaskItem, error) {
	status := normalizeTaskStatus(req.Status)
	db := dao.DB.WithContext(ctx)
	var t model.AdminTask
	if err := db.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("admin: 任务不存在")
		}
		return nil, fmt.Errorf("admin: 查询任务失败: %w", err)
	}
	from := t.Status
	t.Status = status
	if req.OrderIdx != nil {
		t.OrderIdx = *req.OrderIdx
	} else {
		t.OrderIdx = nextOrderIdx(db, status)
	}
	if err := db.Model(&t).Updates(map[string]any{"status": t.Status, "order_idx": t.OrderIdx}).Error; err != nil {
		return nil, fmt.Errorf("admin: 移动任务失败: %w", err)
	}
	if from != status {
		recordActivity(db, t.ID, actorID, actorName, "moved", fmt.Sprintf("%s → %s", statusLabel(from), statusLabel(status)))
	}
	item := toTaskItem(t)
	return &item, nil
}

// DeleteTask 软删除任务并清理其评论、子清单与活动。
func DeleteTask(ctx context.Context, id uint) error {
	return dao.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

// ── 任务评论 ─────────────────────────────────────────────────

// ListTaskComments 返回某任务的评论,按时间升序。
func ListTaskComments(ctx context.Context, taskID uint) (*dto.AdminTaskCommentListResponse, error) {
	db := dao.DB.WithContext(ctx)
	var rows []model.AdminTaskComment
	if err := db.Where("task_id = ?", taskID).Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询任务评论失败: %w", err)
	}
	out := &dto.AdminTaskCommentListResponse{Total: int64(len(rows))}
	for _, r := range rows {
		out.Items = append(out.Items, dto.AdminTaskComment{
			ID:         r.ID,
			TaskID:     r.TaskID,
			AuthorName: r.AuthorName,
			Content:    r.Content,
			CreatedAt:  r.CreatedAt,
		})
	}
	return out, nil
}

// AddTaskComment 给任务追加一条评论。
func AddTaskComment(ctx context.Context, taskID uint, content string, authorID uint, authorName string) (*dto.AdminTaskComment, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("admin: 评论内容不能为空")
	}
	db := dao.DB.WithContext(ctx)
	var cnt int64
	if err := db.Model(&model.AdminTask{}).Where("id = ?", taskID).Count(&cnt).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询任务失败: %w", err)
	}
	if cnt == 0 {
		return nil, fmt.Errorf("admin: 任务不存在")
	}
	c := &model.AdminTaskComment{
		TaskID:     taskID,
		AuthorID:   authorID,
		AuthorName: authorName,
		Content:    content,
	}
	if err := db.Create(c).Error; err != nil {
		return nil, fmt.Errorf("admin: 新增评论失败: %w", err)
	}
	recordActivity(db, taskID, authorID, authorName, "commented", "发表了评论")
	return &dto.AdminTaskComment{
		ID:         c.ID,
		TaskID:     c.TaskID,
		AuthorName: c.AuthorName,
		Content:    c.Content,
		CreatedAt:  c.CreatedAt,
	}, nil
}

// DeleteTaskComment 删除一条评论。
func DeleteTaskComment(ctx context.Context, taskID, commentID uint) error {
	if err := dao.DB.WithContext(ctx).
		Where("id = ? AND task_id = ?", commentID, taskID).
		Delete(&model.AdminTaskComment{}).Error; err != nil {
		return fmt.Errorf("admin: 删除评论失败: %w", err)
	}
	return nil
}

// ── 任务统计 ─────────────────────────────────────────────────

// TaskStats 汇总任务看板顶部的统计数据。
func TaskStats(ctx context.Context) (*dto.AdminTaskStats, error) {
	db := dao.DB.WithContext(ctx)
	out := &dto.AdminTaskStats{
		ByStatus:   make([]dto.AdminTaskStatusCount, 0, len(taskStatusOrder)),
		ByPriority: make([]dto.AdminTaskPriorityCount, 0, 4),
		ByAssignee: make([]dto.AdminTaskAssigneeCount, 0),
	}

	db.Model(&model.AdminTask{}).Count(&out.Total)
	db.Model(&model.AdminTask{}).Where("status = ?", "done").Count(&out.Done)
	out.Open = out.Total - out.Done
	db.Model(&model.AdminTask{}).
		Where("due_at IS NOT NULL AND due_at < ? AND status <> ?", time.Now(), "done").
		Count(&out.Overdue)
	if out.Total > 0 {
		out.CompletedRate = float64(out.Done) / float64(out.Total)
	}

	type statusRow struct {
		Status string
		Count  int64
	}
	var statusRows []statusRow
	db.Model(&model.AdminTask{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statusRows)
	byStatus := make(map[string]int64, len(statusRows))
	for _, r := range statusRows {
		byStatus[r.Status] = r.Count
	}
	for _, s := range taskStatusOrder {
		out.ByStatus = append(out.ByStatus, dto.AdminTaskStatusCount{Status: s.Status, Count: byStatus[s.Status]})
	}

	type prioRow struct {
		Priority string
		Count    int64
	}
	var prioRows []prioRow
	db.Model(&model.AdminTask{}).
		Select("priority, COUNT(*) as count").
		Group("priority").
		Scan(&prioRows)
	byPrio := make(map[string]int64, len(prioRows))
	for _, r := range prioRows {
		byPrio[r.Priority] = r.Count
	}
	for _, p := range []string{"urgent", "high", "medium", "low"} {
		out.ByPriority = append(out.ByPriority, dto.AdminTaskPriorityCount{Priority: p, Count: byPrio[p]})
	}

	type assigneeRow struct {
		Assignee string
		Count    int64
	}
	var assigneeRows []assigneeRow
	db.Model(&model.AdminTask{}).
		Select("assignee, COUNT(*) as count").
		Where("assignee <> '' AND status <> ?", "done").
		Group("assignee").
		Order("count DESC").
		Limit(10).
		Scan(&assigneeRows)
	for _, r := range assigneeRows {
		out.ByAssignee = append(out.ByAssignee, dto.AdminTaskAssigneeCount{Assignee: r.Assignee, Count: r.Count})
	}
	return out, nil
}
