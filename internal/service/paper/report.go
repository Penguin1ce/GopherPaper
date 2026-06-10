package paper

import (
	"context"
	"fmt"
	"time"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/dao"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
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

	reply, err := ai.GenerateReport(ctx, paperID, t)
	if err != nil {
		return nil, err
	}
	rec := &model.PaperReport{PaperID: paperID, ReportType: t, Content: reply.Content}
	if reply.Meta != nil {
		rec.Meta = model.JSONMap(reply.Meta)
	}
	// 落库失败不影响本次返回,锁释放后下次点击再生成即可。
	if err := paperdao.SaveReport(ctx, rec); err != nil {
		zlog.Error("研读报告落库失败", "paper_id", paperID, "type", string(t), "err", err)
	}
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
