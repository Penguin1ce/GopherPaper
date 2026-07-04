package ragtools

import (
	"context"
	"fmt"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/retrieval"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
)

type compareSearchInput struct {
	PaperID string `json:"paper_id" jsonschema:"description=用户本次选中论文列表中的精确论文 ID,required"`
	Query   string `json:"query" jsonschema:"description=只针对该论文要检索的一个问题或主题,使用简洁独立的自然语言 query,required"`
}

type compareSearchOutput struct {
	PaperID    string      `json:"paper_id"`
	PaperTitle string      `json:"paper_title"`
	Hits       []searchHit `json:"hits" jsonschema:"description=该论文内命中的正文证据;为空时应调整 query 重试或标记证据不足"`
}

// CompareAll 返回多论文对比专用工具。工具要求模型显式指定白名单内的论文 ID,
// 从而保证每轮召回只来自目标论文，避免跨论文检索后把证据归错主体。
func CompareAll() []tool.Tool {
	return []tool.Tool{SearchComparePaper()}
}

// SearchComparePaper 在本次选中论文集合内按单篇检索正文。
func SearchComparePaper() tool.Tool {
	fn := func(ctx context.Context, in compareSearchInput) (compareSearchOutput, error) {
		in.PaperID = strings.TrimSpace(in.PaperID)
		in.Query = strings.TrimSpace(in.Query)
		paper, ok := core.ComparePaperFrom(ctx, in.PaperID)
		if !ok {
			return compareSearchOutput{}, fmt.Errorf("search_compare_paper: 论文不在本次对比范围内")
		}
		if in.Query == "" {
			return compareSearchOutput{}, fmt.Errorf("search_compare_paper: query 不能为空")
		}

		owner := tenant.MustStudentID(ctx)
		docs, err := retrieval.RetrieveForPaper(ctx, in.Query, owner, paper.ID)
		if err != nil {
			return compareSearchOutput{}, fmt.Errorf("search_compare_paper: 检索失败: %w", err)
		}
		docs = retrieval.DropImageDocs(docs)
		retrieval.AddRefs(ctx, retrieval.References(docs))
		zlog.Debug("多论文对比正文检索",
			"paper_id", paper.ID,
			"paper_title", paper.Title,
			"query", in.Query,
			"hits", len(docs),
		)

		hits := make([]searchHit, 0, len(docs))
		for _, doc := range docs {
			ref := retrieval.ReferenceFromDocument(doc)
			hits = append(hits, searchHit{
				Content:     doc.Content,
				Source:      retrieval.FormatReference(ref),
				CitationTag: ref.CitationTag,
				Score:       doc.Score,
			})
		}
		return compareSearchOutput{
			PaperID:    paper.ID,
			PaperTitle: paper.Title,
			Hits:       hits,
		}, nil
	}

	return function.NewFunctionTool(
		fn,
		function.WithName("search_compare_paper"),
		function.WithDescription("在本次多论文对比所选集合中检索某一篇论文的正文证据。每次必须传入选中列表里的精确 paper_id；研究问题、方法、实验设置、数据集与指标、结果、创新和局限应分主题多次检索。返回内容只属于指定论文，引用事实时原样保留 citation_tag。"),
	)
}
