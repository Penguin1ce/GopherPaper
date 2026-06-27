package paper

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/dao"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
	"GopherPaper/internal/service/metrics"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

// reportLockTTL 是单类报告生成锁的存活时间，须长于一次带多模态 RAG 的报告生成耗时，
// 进程崩溃后到期自动释放，可重新生成。
const reportLockTTL = 5 * time.Minute

// Report 取某篇论文某类研读报告,仅限本人。
// 命中持久化缓存直接复用;未命中不在 HTTP 链路同步生成,而是触发一次后台预生成并返回
// ErrReportGenerating,由前端轮询直到命中缓存,避免把整段生成耗时压在这次请求上。
func Report(ctx context.Context, ownerID, paperID string, t constant.ReportType) (*core.Reply, error) {
	if _, err := owned(ctx, ownerID, paperID); err != nil {
		return nil, err
	}
	if reply, ok, err := cachedReport(ctx, paperID, t); err != nil {
		return nil, err
	} else if ok {
		return reply, nil
	}
	// 触发后台生成,生成锁在 worker 内保证只写一次,重复投递只会被去重为空操作。
	enqueueReport(ctx, reportTask{PaperID: paperID, OwnerID: ownerID, ReportType: t})
	return nil, errs.ErrReportGenerating
}

// ReadyReports 列出某篇论文已生成的研读报告类型,仅限本人,供前端进入时回填就绪态。
// 只读,不触发任何生成。先读 Redis 缓存(生成期前端会 5 秒轮询,避免一直打 MySQL),
// 未命中再查 DB 并回填缓存;缓存由 SaveReport 主动失效,故不会漏掉新生成的报告。
func ReadyReports(ctx context.Context, ownerID, paperID string) ([]constant.ReportType, error) {
	if _, err := owned(ctx, ownerID, paperID); err != nil {
		return nil, err
	}
	cacheKey := constant.ReportReadyCacheKeyPrefix + paperID
	if cached, err := dao.Get(ctx, cacheKey); err == nil {
		var types []constant.ReportType
		if json.Unmarshal([]byte(cached), &types) == nil {
			return types, nil
		}
		// 缓存内容损坏不致命,落到 DB 重建。
	}
	types, err := paperdao.ListReportTypes(ctx, paperID)
	if err != nil {
		return nil, err
	}
	// 回填缓存(空数组也缓存,区分未缓存与确无报告;写失败不影响本次返回)。
	if blob, mErr := json.Marshal(types); mErr == nil {
		if sErr := dao.SetTTL(ctx, cacheKey, blob, constant.ReportReadyCacheTTL); sErr != nil {
			zlog.Error("就绪报告缓存写入失败", "paper_id", paperID, "err", sErr)
		}
	}
	return types, nil
}

// invalidateReadyCache 失效某篇论文的就绪报告缓存,写入新报告后调用,下次查询从 DB 重建。
func invalidateReadyCache(ctx context.Context, paperID string) {
	if _, err := dao.Del(context.WithoutCancel(ctx), constant.ReportReadyCacheKeyPrefix+paperID); err != nil {
		zlog.Error("就绪报告缓存失效失败", "paper_id", paperID, "err", err)
	}
}

// ensureReport 保证某类报告存在并返回:命中缓存即复用,否则抢 Redis 锁后生成并落库。
// 抢锁失败说明已有 worker 或并发点击在生成,返回 ErrReportGenerating,只允许一次写入。
// 调用前须保证 ctx 已注入 owner 租户,供 ai.GenerateReport 选到该用户模型。
func ensureReport(ctx context.Context, paperID string, t constant.ReportType) (*core.Reply, error) {
	if reply, ok, err := cachedReport(ctx, paperID, t); err != nil {
		return nil, err
	} else if ok {
		return reply, nil
	}

	lockKey := fmt.Sprintf("report:lock:%s:%s", paperID, t)
	got, err := dao.SetNX(ctx, lockKey, "1", reportLockTTL)
	if err != nil {
		return nil, fmt.Errorf("service/paper: 申请报告锁失败: %w", err)
	}
	if !got {
		// 别人正在生成,不重复生成与写入。
		return nil, errs.ErrReportGenerating
	}
	defer func() { _, _ = dao.Del(context.WithoutCancel(ctx), lockKey) }()

	// 双检:抢到锁后可能别人刚写完缓存,直接复用避免重算。
	if reply, ok, err := cachedReport(ctx, paperID, t); err != nil {
		return nil, err
	} else if ok {
		return reply, nil
	}

	start := time.Now()
	ownerID := tenant.MustStudentID(ctx)
	metricSuccess := false
	var metricErr error
	defer func() {
		metrics.Record(ctx, metrics.ServiceReport, ownerID, paperID, "", metricSuccess, time.Since(start), metricErr)
	}()

	reply, err := ai.GenerateReport(ctx, paperID, t)
	if err != nil {
		metricErr = err
		return nil, err
	}
	rec := &model.PaperReport{PaperID: paperID, ReportType: t, Content: reply.Content}
	if reply.Meta != nil {
		rec.Meta = model.JSONMap(reply.Meta)
	}
	if err := paperdao.SaveReport(ctx, rec); err != nil {
		zlog.Error("研读报告落库失败", "paper_id", paperID, "type", string(t), "err", err)
		metricErr = err
		return nil, err
	}
	// 落库成功才失效就绪缓存,让轮询/重开下一次查询看到这条新报告。
	invalidateReadyCache(ctx, paperID)
	metricSuccess = true
	return reply, nil
}

// cachedReport 查报告持久化缓存,命中返回 Reply 与 true,未命中返回 false。
func cachedReport(ctx context.Context, paperID string, t constant.ReportType) (*core.Reply, bool, error) {
	cached, err := paperdao.GetReport(ctx, paperID, t)
	if err == errs.ErrReportNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &core.Reply{Content: cached.Content, Meta: cached.Meta}, true, nil
}
