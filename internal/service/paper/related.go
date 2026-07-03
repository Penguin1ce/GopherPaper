package paper

import (
	"context"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/parser"
	"GopherPaper/internal/zlog"
)

func generateRelatedResearch(ctx context.Context, paperID string) (*core.Reply, error) {
	p, err := paperdao.Get(ctx, paperID)
	if err != nil {
		return nil, err
	}
	refs := parsedReferencesForPaper(paperID)
	return ai.GenerateRelatedResearch(ctx, p.Title, refs)
}

func parsedReferencesForPaper(paperID string) []string {
	doc, err := parser.ParseArtifactDir(mineruDir(paperID))
	if err != nil {
		zlog.Warn("读取 MinerU 参考文献失败,相关研究降级按标题查找", "paper_id", paperID, "err", err)
		return nil
	}
	return doc.References
}
