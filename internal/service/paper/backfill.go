package paper

import (
	"context"
	"errors"

	"GopherPaper/internal/ai/core"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/graph"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

// BackfillGraph 把存量已就绪论文回填进知识图谱:遍历 ready 论文,按其元信息建论文与
// 作者/关键词/机构/会议节点。参考文献只存在于解析中间产物未落库,故回填不重建引用关系,
// 引用边仅对回填后新解析的论文生成。供 cmd/server 的 -backfill-graph 一次性入口调用。
func BackfillGraph(ctx context.Context) error {
	papers, err := paperdao.ListByStatus(ctx, constant.PaperReady)
	if err != nil {
		return err
	}
	var done, skipped int
	owners := map[string]bool{}
	for _, p := range papers {
		meta, err := paperdao.GetMeta(ctx, p.ID)
		if err != nil {
			if errors.Is(err, errs.ErrPaperNotFound) {
				skipped++
				continue
			}
			return err
		}
		title := p.Title
		if title == "" {
			title = meta.PaperID
		}
		// 注入论文 owner,与解析链路一致,虽 graph.UpsertPaper 不依赖 ctx 身份,保持上下文统一。
		uctx := tenant.With(ctx, tenant.Tenant{StudentID: p.OwnerID})
		if err := graph.UpsertPaper(uctx, graph.PaperGraph{
			Owner:        p.OwnerID,
			ID:           p.ID,
			Title:        title,
			Year:         meta.PublishYear,
			Venue:        meta.Venue,
			Authors:      meta.Authors,
			Keywords:     meta.Keywords,
			Affiliations: meta.Affiliations,
			Embedding: paperEmbedding(uctx, &core.PaperStructured{
				Title:             title,
				Abstract:          meta.Abstract,
				Keywords:          meta.Keywords,
				ResearchQuestions: meta.ResearchQuestions,
				Methods:           meta.Methods,
				Innovations:       meta.Innovations,
			}),
		}); err != nil {
			zlog.Error("回填图谱失败,跳过", "paper_id", p.ID, "err", err)
			skipped++
			continue
		}
		owners[p.OwnerID] = true
		done++
	}
	// 归一化键变更可能留下旧 name 键的孤立节点,按 owner 清一遍。
	for owner := range owners {
		if err := graph.CleanupOrphans(ctx, owner); err != nil {
			zlog.Error("回填后清理孤立节点失败", "owner", owner, "err", err)
		}
	}
	zlog.Info("知识图谱回填完成", "done", done, "skipped", skipped, "total", len(papers))
	return nil
}
