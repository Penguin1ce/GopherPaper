package admin

import (
	"context"
	"fmt"
	"sort"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/graph"
	"GopherPaper/internal/knowledge"
	"GopherPaper/internal/model"
)

// Analytics 汇总看板所需的近 N 天时间序列、论文状态分布与服务调用统计。
func Analytics(ctx context.Context, days int) (*dto.AdminAnalytics, error) {
	if days <= 0 || days > 180 {
		days = 30
	}
	out := &dto.AdminAnalytics{Days: days}
	db := dao.DB.WithContext(ctx)

	db.Model(&model.Paper{}).Count(&out.TotalPapers)
	db.Model(&model.User{}).Count(&out.TotalUsers)
	db.Model(&model.ServiceCallLog{}).Count(&out.TotalCalls)
	db.Model(&model.Session{}).Count(&out.TotalSessions)

	now := time.Now()
	start := now.AddDate(0, 0, -(days - 1))
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())

	trend := make([]dto.AdminTrendPoint, days)
	idx := make(map[string]*dto.AdminTrendPoint, days)
	for i := 0; i < days; i++ {
		d := startDay.AddDate(0, 0, i).Format("2006-01-02")
		trend[i].Date = d
	}
	for i := range trend {
		idx[trend[i].Date] = &trend[i]
	}

	type bucketRow struct {
		Bucket string
		Total  int64
	}
	fillCount := func(model any, target func(*dto.AdminTrendPoint, int64)) {
		var rows []bucketRow
		db.Model(model).
			Select("DATE_FORMAT(created_at, '%Y-%m-%d') as bucket, COUNT(*) as total").
			Where("created_at >= ?", startDay).
			Group("bucket").
			Scan(&rows)
		for _, r := range rows {
			if p := idx[r.Bucket]; p != nil {
				target(p, r.Total)
			}
		}
	}
	fillCount(&model.Paper{}, func(p *dto.AdminTrendPoint, v int64) { p.Papers = v })
	fillCount(&model.User{}, func(p *dto.AdminTrendPoint, v int64) { p.Users = v })
	fillCount(&model.Session{}, func(p *dto.AdminTrendPoint, v int64) { p.Sessions = v })

	type callRow struct {
		Bucket  string
		Total   int64
		Success int64
		AvgMs   float64
	}
	var callRows []callRow
	db.Model(&model.ServiceCallLog{}).
		Select("DATE_FORMAT(created_at, '%Y-%m-%d') as bucket, COUNT(*) as total, SUM(CASE WHEN success THEN 1 ELSE 0 END) as success, AVG(duration_ms) as avg_ms").
		Where("created_at >= ?", startDay).
		Group("bucket").
		Scan(&callRows)
	for _, r := range callRows {
		if p := idx[r.Bucket]; p != nil {
			p.Calls = r.Total
			p.Success = r.Success
			p.Failed = r.Total - r.Success
			p.AvgLatencyMS = r.AvgMs
		}
	}
	out.Trend = trend

	type statusRow struct {
		Status string
		Total  int64
	}
	var statusRows []statusRow
	db.Model(&model.Paper{}).
		Select("status, COUNT(*) as total").
		Group("status").
		Scan(&statusRows)
	for _, r := range statusRows {
		out.StatusDist = append(out.StatusDist, dto.AdminStatusSlice{Status: r.Status, Count: r.Total})
	}

	type svcRow struct {
		ServiceType string
		Total       int64
		Success     int64
		AvgMs       float64
		MaxMs       int64
	}
	var svcRows []svcRow
	db.Model(&model.ServiceCallLog{}).
		Select("service_type, COUNT(*) as total, SUM(CASE WHEN success THEN 1 ELSE 0 END) as success, AVG(duration_ms) as avg_ms, MAX(duration_ms) as max_ms").
		Group("service_type").
		Order("total DESC").
		Scan(&svcRows)
	for _, r := range svcRows {
		out.Services = append(out.Services, dto.AdminServiceStat{
			ServiceType: r.ServiceType,
			Total:       r.Total,
			Success:     r.Success,
			Failed:      r.Total - r.Success,
			AvgMS:       r.AvgMs,
			MaxMS:       r.MaxMs,
		})
	}

	return out, nil
}

// Health 逐个探活后台依赖的中间件,返回在线状态与延迟,支撑运维大盘。
func Health(ctx context.Context) *dto.AdminHealth {
	out := &dto.AdminHealth{CheckedAt: time.Now()}
	add := func(name string, err error, elapsed time.Duration, detail string) {
		item := dto.AdminHealthItem{Name: name, LatencyMS: elapsed.Milliseconds(), Detail: detail}
		if err != nil {
			item.Status = "down"
			item.Detail = truncateStr(err.Error(), 200)
		} else {
			item.Status = "up"
		}
		out.Items = append(out.Items, item)
		out.Total++
		if err == nil {
			out.Healthy++
		}
	}

	probe := func(fn func(context.Context) error) (error, time.Duration) {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		t := time.Now()
		return fn(cctx), time.Since(t)
	}

	err, d := probe(func(c context.Context) error {
		if dao.DB == nil {
			return fmt.Errorf("mysql 未初始化")
		}
		var one int
		return dao.DB.WithContext(c).Raw("SELECT 1").Scan(&one).Error
	})
	add("MySQL", err, d, "")

	err, d = probe(func(c context.Context) error {
		if dao.RDB == nil {
			return fmt.Errorf("redis 未初始化")
		}
		return dao.RDB.Ping(c).Err()
	})
	add("Redis", err, d, "")

	var vectorDetail string
	err, d = probe(func(c context.Context) error {
		n, e := knowledge.VectorCount(c)
		if e == nil {
			vectorDetail = fmt.Sprintf("%d vectors", n)
		}
		return e
	})
	add("Milvus", err, d, vectorDetail)

	err, d = probe(graph.Ping)
	add("Neo4j", err, d, "")

	err, d = probe(func(context.Context) error { return pingRabbit() })
	add("RabbitMQ", err, d, "")

	return out
}

func pingRabbit() error {
	if mqURL == "" {
		return fmt.Errorf("rabbitmq url 未配置")
	}
	conn, err := amqp.DialConfig(mqURL, amqp.Config{Dial: amqp.DefaultDial(3 * time.Second)})
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// Activity 汇总最近的论文上传、用户注册与服务调用,按时间倒序合并为活动流。
func Activity(ctx context.Context, limit int) (*dto.AdminActivity, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	out := &dto.AdminActivity{}
	db := dao.DB.WithContext(ctx)

	var papers []model.Paper
	db.Order("created_at DESC").Limit(limit).Find(&papers)
	for _, p := range papers {
		out.Items = append(out.Items, dto.AdminActivityItem{
			Type:      "paper",
			Title:     paperDisplayTitle(p),
			Subtitle:  "owner " + p.OwnerID,
			Status:    string(p.Status),
			CreatedAt: p.CreatedAt,
		})
	}

	var users []model.User
	db.Order("created_at DESC").Limit(limit).Find(&users)
	for _, u := range users {
		name := u.Name
		if name == "" {
			name = u.StudentID
		}
		out.Items = append(out.Items, dto.AdminActivityItem{
			Type:      "user",
			Title:     name,
			Subtitle:  u.Email,
			CreatedAt: u.CreatedAt,
		})
	}

	var calls []model.ServiceCallLog
	db.Order("created_at DESC").Limit(limit).Find(&calls)
	for _, c := range calls {
		status := "success"
		if !c.Success {
			status = "failed"
		}
		out.Items = append(out.Items, dto.AdminActivityItem{
			Type:      "call",
			Title:     c.ServiceType,
			Subtitle:  fmt.Sprintf("%d ms", c.DurationMS),
			Status:    status,
			CreatedAt: c.CreatedAt,
		})
	}

	sort.SliceStable(out.Items, func(i, j int) bool {
		return out.Items[i].CreatedAt.After(out.Items[j].CreatedAt)
	})
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
	}
	return out, nil
}

func paperDisplayTitle(p model.Paper) string {
	if p.Title != "" {
		return p.Title
	}
	if p.FileName != "" {
		return p.FileName
	}
	return p.ID
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
