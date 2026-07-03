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

// ListUsers 分页返回用户列表,支持按姓名/学号/邮箱搜索与班级筛选,
// 并为每个用户批量填充论文数、会话数、调用数与最近活跃时间。
func ListUsers(ctx context.Context, query, classID string, page, pageSize int) (*dto.AdminUserListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	db := dao.DB.WithContext(ctx)

	base := db.Model(&model.User{})
	if q := strings.TrimSpace(query); q != "" {
		like := "%" + q + "%"
		base = base.Where("name LIKE ? OR student_id LIKE ? OR email LIKE ?", like, like, like)
	}
	if c := strings.TrimSpace(classID); c != "" {
		base = base.Where("class_id = ?", c)
	}

	out := &dto.AdminUserListResponse{Page: page, PageSize: pageSize}
	if err := base.Count(&out.Total).Error; err != nil {
		return nil, fmt.Errorf("admin: 统计用户失败: %w", err)
	}

	var users []model.User
	if err := base.
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&users).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询用户失败: %w", err)
	}
	if len(users) == 0 {
		out.Items = []dto.AdminUserItem{}
		return out, nil
	}

	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.StudentID)
	}

	paperCounts := countByKey(db, &model.Paper{}, "owner_id", ids)
	sessionCounts := countByKey(db, &model.Session{}, "student_id", ids)
	callCounts := countByKey(db, &model.ServiceCallLog{}, "actor_id", ids)
	lastActive := lastActiveByActor(db, ids)

	out.Items = make([]dto.AdminUserItem, 0, len(users))
	for _, u := range users {
		item := dto.AdminUserItem{
			ID:           u.ID,
			StudentID:    u.StudentID,
			Name:         u.Name,
			Email:        u.Email,
			ClassID:      u.ClassID,
			AvatarURL:    u.AvatarURL,
			PaperCount:   paperCounts[u.StudentID],
			SessionCount: sessionCounts[u.StudentID],
			CallCount:    callCounts[u.StudentID],
			CreatedAt:    u.CreatedAt,
		}
		if t, ok := lastActive[u.StudentID]; ok {
			tt := t
			item.LastActiveAt = &tt
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

// countByKey 按给定列在 ids 范围内分组计数,返回 key->count 映射。
func countByKey(db *gorm.DB, model any, column string, ids []string) map[string]int64 {
	type kc struct {
		K string
		C int64
	}
	var rows []kc
	db.Model(model).
		Select(column+" as k, COUNT(*) as c").
		Where(column+" IN ?", ids).
		Group(column).
		Scan(&rows)
	m := make(map[string]int64, len(rows))
	for _, r := range rows {
		m[r.K] = r.C
	}
	return m
}

// lastActiveByActor 取每个 actor 最近一次服务调用时间,作为"最近活跃"近似。
func lastActiveByActor(db *gorm.DB, ids []string) map[string]time.Time {
	type kt struct {
		K string
		T time.Time
	}
	var rows []kt
	db.Model(&model.ServiceCallLog{}).
		Select("actor_id as k, MAX(created_at) as t").
		Where("actor_id IN ?", ids).
		Group("actor_id").
		Scan(&rows)
	m := make(map[string]time.Time, len(rows))
	for _, r := range rows {
		m[r.K] = r.T
	}
	return m
}

// GetUserDetail 返回单个用户的画像:基础信息、论文状态分布、近 14 天活动趋势与最近论文。
func GetUserDetail(ctx context.Context, id uint) (*dto.AdminUserDetail, error) {
	db := dao.DB.WithContext(ctx)

	var user model.User
	if err := db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("admin: 用户不存在")
		}
		return nil, fmt.Errorf("admin: 查询用户失败: %w", err)
	}

	detail := &dto.AdminUserDetail{}
	detail.User = dto.AdminUserItem{
		ID:        user.ID,
		StudentID: user.StudentID,
		Name:      user.Name,
		Email:     user.Email,
		ClassID:   user.ClassID,
		AvatarURL: user.AvatarURL,
		CreatedAt: user.CreatedAt,
	}
	db.Model(&model.Paper{}).Where("owner_id = ?", user.StudentID).Count(&detail.User.PaperCount)
	db.Model(&model.Session{}).Where("student_id = ?", user.StudentID).Count(&detail.User.SessionCount)
	db.Model(&model.ServiceCallLog{}).Where("actor_id = ?", user.StudentID).Count(&detail.User.CallCount)

	// 论文状态分布
	type sc struct {
		Status string
		Total  int64
	}
	var scs []sc
	db.Model(&model.Paper{}).
		Select("status, COUNT(*) as total").
		Where("owner_id = ?", user.StudentID).
		Group("status").
		Scan(&scs)
	for _, s := range scs {
		detail.PaperStatusDist = append(detail.PaperStatusDist, dto.AdminUserStatusCount{Status: s.Status, Count: s.Total})
	}

	// 近 14 天活动趋势
	detail.Activity = userActivityTrend(db, user.StudentID, 14)

	// 最近论文
	var papers []model.Paper
	db.Where("owner_id = ?", user.StudentID).Order("created_at DESC").Limit(10).Find(&papers)
	for _, p := range papers {
		detail.RecentPapers = append(detail.RecentPapers, dto.AdminPaperItem{
			ID:        p.ID,
			OwnerID:   p.OwnerID,
			Title:     p.Title,
			FileName:  p.FileName,
			Size:      p.Size,
			Status:    string(p.Status),
			PageCount: p.PageCount,
			CreatedAt: p.CreatedAt,
			UpdatedAt: p.UpdatedAt,
		})
	}

	return detail, nil
}

// userActivityTrend 汇总某用户最近 days 天每日的论文/会话/调用数。
func userActivityTrend(db *gorm.DB, studentID string, days int) []dto.AdminUserActivityPoint {
	if days < 1 {
		days = 14
	}
	now := time.Now()
	start := now.AddDate(0, 0, -(days - 1))
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())

	points := make([]dto.AdminUserActivityPoint, days)
	idx := make(map[string]*dto.AdminUserActivityPoint, days)
	for i := 0; i < days; i++ {
		points[i].Date = startDay.AddDate(0, 0, i).Format("2006-01-02")
	}
	for i := range points {
		idx[points[i].Date] = &points[i]
	}

	type br struct {
		Bucket string
		Total  int64
	}
	fill := func(m any, col string, set func(*dto.AdminUserActivityPoint, int64)) {
		var rows []br
		db.Model(m).
			Select("DATE_FORMAT(created_at, '%Y-%m-%d') as bucket, COUNT(*) as total").
			Where(col+" = ? AND created_at >= ?", studentID, startDay).
			Group("bucket").
			Scan(&rows)
		for _, r := range rows {
			if p := idx[r.Bucket]; p != nil {
				set(p, r.Total)
			}
		}
	}
	fill(&model.Paper{}, "owner_id", func(p *dto.AdminUserActivityPoint, v int64) { p.Papers = v })
	fill(&model.Session{}, "student_id", func(p *dto.AdminUserActivityPoint, v int64) { p.Sessions = v })
	fill(&model.ServiceCallLog{}, "actor_id", func(p *dto.AdminUserActivityPoint, v int64) { p.Calls = v })
	return points
}

// ClassStats 按班级聚合用户数与论文数,用于班级维度概览。
func ClassStats(ctx context.Context) (*dto.AdminClassStatsResponse, error) {
	db := dao.DB.WithContext(ctx)
	out := &dto.AdminClassStatsResponse{}

	type cu struct {
		ClassID string
		Total   int64
	}
	var userRows []cu
	db.Model(&model.User{}).
		Select("class_id, COUNT(*) as total").
		Group("class_id").
		Scan(&userRows)

	// 班级->论文数:用户表 join 论文表(按 owner_id=student_id)
	type cp struct {
		ClassID string
		Total   int64
	}
	var paperRows []cp
	db.Model(&model.User{}).
		Select("users.class_id as class_id, COUNT(papers.id) as total").
		Joins("LEFT JOIN papers ON papers.owner_id = users.student_id AND papers.deleted_at IS NULL").
		Group("users.class_id").
		Scan(&paperRows)
	paperByClass := make(map[string]int64, len(paperRows))
	for _, r := range paperRows {
		paperByClass[r.ClassID] = r.Total
	}

	for _, r := range userRows {
		label := r.ClassID
		if label == "" {
			label = "未分班"
		}
		out.Items = append(out.Items, dto.AdminClassStat{
			ClassID:    label,
			UserCount:  r.Total,
			PaperCount: paperByClass[r.ClassID],
		})
	}
	return out, nil
}
