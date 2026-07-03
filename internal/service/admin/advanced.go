package admin

import (
	"context"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
)

// latencyBuckets 定义延迟直方图的分档边界(毫秒),右开区间。
var latencyBuckets = []struct {
	Label string
	Min   int64
	Max   int64 // -1 表示无上界
}{
	{"<0.5s", 0, 500},
	{"0.5–1s", 500, 1000},
	{"1–2s", 1000, 2000},
	{"2–3s", 2000, 3000},
	{"3–5s", 3000, 5000},
	{"≥5s", 5000, -1},
}

// pipelineOrder 是论文流水线阶段的展示顺序。
var pipelineOrder = []constant.PaperStatus{
	constant.PaperUploaded,
	constant.PaperParsing,
	constant.PaperExtracted,
	constant.PaperIndexed,
	constant.PaperReady,
	constant.PaperFailed,
}

// AdvancedAnalytics 汇总延迟直方图、按小时分布、Top 用户与流水线漏斗。
func AdvancedAnalytics(ctx context.Context) (*dto.AdminAdvancedAnalytics, error) {
	db := dao.DB.WithContext(ctx)
	out := &dto.AdminAdvancedAnalytics{}

	// 延迟直方图:逐档计数
	for _, b := range latencyBuckets {
		q := db.Model(&model.ServiceCallLog{}).Where("duration_ms >= ?", b.Min)
		if b.Max >= 0 {
			q = q.Where("duration_ms < ?", b.Max)
		}
		var c int64
		q.Count(&c)
		out.LatencyHistogram = append(out.LatencyHistogram, dto.AdminLatencyBucket{Label: b.Label, Count: c})
	}

	// 按小时分布(0-23)
	type hourRow struct {
		Hour    int
		Total   int64
		Success int64
	}
	var hrs []hourRow
	db.Model(&model.ServiceCallLog{}).
		Select("HOUR(created_at) as hour, COUNT(*) as total, SUM(CASE WHEN success THEN 1 ELSE 0 END) as success").
		Group("hour").
		Scan(&hrs)
	hourMap := make(map[int]hourRow, 24)
	for _, r := range hrs {
		hourMap[r.Hour] = r
	}
	for h := 0; h < 24; h++ {
		r := hourMap[h]
		out.HourlyDistribution = append(out.HourlyDistribution, dto.AdminHourPoint{
			Hour:    h,
			Count:   r.Total,
			Success: r.Success,
		})
	}

	// Top 调用用户
	type actorRow struct {
		ActorID string
		Calls   int64
	}
	var actors []actorRow
	db.Model(&model.ServiceCallLog{}).
		Select("actor_id, COUNT(*) as calls").
		Where("actor_id <> ''").
		Group("actor_id").
		Order("calls DESC").
		Limit(10).
		Scan(&actors)
	for _, a := range actors {
		out.TopActors = append(out.TopActors, dto.AdminTopActor{ActorID: a.ActorID, Calls: a.Calls})
	}

	// 流水线漏斗:各状态论文数,按流水线顺序
	type statusRow struct {
		Status string
		Total  int64
	}
	var srs []statusRow
	db.Model(&model.Paper{}).
		Select("status, COUNT(*) as total").
		Group("status").
		Scan(&srs)
	statusMap := make(map[string]int64, len(srs))
	for _, s := range srs {
		statusMap[s.Status] = s.Total
	}
	for _, st := range pipelineOrder {
		out.Pipeline = append(out.Pipeline, dto.AdminPipelineStage{
			Stage: string(st),
			Count: statusMap[string(st)],
		})
	}

	return out, nil
}
