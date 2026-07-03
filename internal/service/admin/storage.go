package admin

import (
	"context"
	"fmt"
	"time"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
)

// storageBucketDefs 按单文件体积从小到大分档,上界单位为字节,最后一档上界为 0 表示无上界。
var storageBucketDefs = []struct {
	Label string
	Upper int64
}{
	{"< 1 MB", 1 << 20},
	{"1–5 MB", 5 << 20},
	{"5–10 MB", 10 << 20},
	{"10–20 MB", 20 << 20},
	{"> 20 MB", 0},
}

// StorageOverview 聚合论文文件的存储占用:总量、按状态/体积分档/月份的分布以及体积 Top 榜。
// 全部基于 papers 表的 size 字段,不触碰真实文件系统,可安全在后台频繁刷新。
func StorageOverview(ctx context.Context, topN int) (*dto.AdminStorageOverview, error) {
	if topN <= 0 || topN > 50 {
		topN = 10
	}
	db := dao.DB.WithContext(ctx)
	out := &dto.AdminStorageOverview{}

	// 总量:数量 / 总体积 / 总页数。
	var agg struct {
		Papers int64
		Bytes  int64
		Pages  int64
	}
	if err := db.Model(&model.Paper{}).
		Select("COUNT(*) as papers, COALESCE(SUM(size),0) as bytes, COALESCE(SUM(page_count),0) as pages").
		Scan(&agg).Error; err != nil {
		return nil, fmt.Errorf("admin: 统计存储总量失败: %w", err)
	}
	out.TotalPapers = agg.Papers
	out.TotalBytes = agg.Bytes
	out.TotalPages = agg.Pages
	if agg.Papers > 0 {
		out.AvgBytes = agg.Bytes / agg.Papers
	}
	db.Model(&model.Paper{}).Where("size <= 0").Count(&out.MissingSize)

	// 按状态分档。
	type statusRow struct {
		Status string
		Count  int64
		Bytes  int64
	}
	var statusRows []statusRow
	if err := db.Model(&model.Paper{}).
		Select("status, COUNT(*) as count, COALESCE(SUM(size),0) as bytes").
		Group("status").
		Order("bytes DESC").
		Scan(&statusRows).Error; err != nil {
		return nil, fmt.Errorf("admin: 统计存储状态分布失败: %w", err)
	}
	for _, r := range statusRows {
		out.ByStatus = append(out.ByStatus, dto.AdminStorageStatusUsage{
			Status: r.Status,
			Count:  r.Count,
			Bytes:  r.Bytes,
		})
	}

	// 按单文件体积分档。
	out.Buckets = make([]dto.AdminStorageBucket, len(storageBucketDefs))
	var lower int64
	for i, def := range storageBucketDefs {
		q := db.Model(&model.Paper{}).Where("size > ?", lower)
		if def.Upper > 0 {
			q = q.Where("size <= ?", def.Upper)
		}
		var row struct {
			Count int64
			Bytes int64
		}
		q.Select("COUNT(*) as count, COALESCE(SUM(size),0) as bytes").Scan(&row)
		out.Buckets[i] = dto.AdminStorageBucket{Label: def.Label, Count: row.Count, Bytes: row.Bytes}
		lower = def.Upper
	}

	// 近 12 个月上传体积趋势(补齐空月份)。
	now := time.Now()
	trend := make([]dto.AdminStorageTrendPoint, 12)
	idx := make(map[string]*dto.AdminStorageTrendPoint, 12)
	for i := 0; i < 12; i++ {
		m := now.AddDate(0, -(11 - i), 0).Format("2006-01")
		trend[i].Month = m
		idx[trend[i].Month] = &trend[i]
	}
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -11, 0)
	type trendRow struct {
		Month string
		Count int64
		Bytes int64
	}
	var trendRows []trendRow
	db.Model(&model.Paper{}).
		Select("DATE_FORMAT(created_at, '%Y-%m') as month, COUNT(*) as count, COALESCE(SUM(size),0) as bytes").
		Where("created_at >= ?", start).
		Group("month").
		Scan(&trendRows)
	for _, r := range trendRows {
		if p := idx[r.Month]; p != nil {
			p.Count = r.Count
			p.Bytes = r.Bytes
		}
	}
	out.Trend = trend

	// 体积 Top 榜。
	var top []model.Paper
	if err := db.Model(&model.Paper{}).
		Order("size DESC").
		Limit(topN).
		Find(&top).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询存储 Top 榜失败: %w", err)
	}
	for _, p := range top {
		out.TopPapers = append(out.TopPapers, dto.AdminStorageTopPaper{
			ID:        p.ID,
			Title:     p.Title,
			FileName:  p.FileName,
			OwnerID:   p.OwnerID,
			Status:    string(p.Status),
			Size:      p.Size,
			PageCount: p.PageCount,
			CreatedAt: p.CreatedAt,
		})
	}
	return out, nil
}
