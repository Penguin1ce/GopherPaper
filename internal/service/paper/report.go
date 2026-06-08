package paper

import (
	"context"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

// Report 生成某篇论文某类研读报告,仅限本人。
// 命中持久化缓存直接复用,否则调模型生成后落库再返回,第二次点击同一按钮即取缓存不重算。
func Report(ctx context.Context, ownerID, paperID string, t constant.ReportType) (*core.Reply, error) {
	if _, err := owned(ctx, ownerID, paperID); err != nil {
		return nil, err
	}
	if cached, err := paperdao.GetReport(ctx, paperID, t); err == nil {
		return &core.Reply{Content: cached.Content, Meta: cached.Meta}, nil
	} else if err != errs.ErrReportNotFound {
		return nil, err
	}
	reply, err := ai.GenerateReport(ctx, paperID, t)
	if err != nil {
		return nil, err
	}
	rec := &model.PaperReport{PaperID: paperID, ReportType: t, Content: reply.Content}
	if reply.Meta != nil {
		rec.Meta = model.JSONMap(reply.Meta)
	}
	// 落库失败不影响本次返回,下次点击再生成即可。
	if err := paperdao.SaveReport(ctx, rec); err != nil {
		zlog.Error("研读报告落库失败", "paper_id", paperID, "type", string(t), "err", err)
	}
	return reply, nil
}
