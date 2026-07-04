package ai

import (
	"context"

	"GopherPaper/internal/ai/core"
	gopherflow "GopherPaper/internal/ai/gopher"
)

// ComparePapers 经小囊鼠的逐篇检索、对齐写作和交叉审校流水线生成横向对比分析。
func ComparePapers(ctx context.Context, papers []core.PaperCompareInput) (*core.Reply, error) {
	return gopherflow.Compare(ctx, papers)
}
