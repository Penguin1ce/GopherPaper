// Package ragtools 是 agentic agent 共用的论文检索工具:身份与论文范围全部从 ctx 取,模型不传 id。
// 与 toolkit 的 search_my_papers 区别:这两个工具自动限定到当前会话绑定的论文(core.PaperIDFrom),
// 并把命中出处写进 ctx 的引用收集器(retrieval.AddRefs),供循环结束后汇成 Reply.Meta["sources"]。
// 抽成叶子包供问答(ragagent)与研读报告(gopher)两条 agentic 链路复用,互不耦合。
package ragtools

import (
	"context"
	"fmt"
	"path/filepath"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/retrieval"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// All 返回 agentic 链路的全部检索工具:正文检索与图表检索。
func All() []tool.Tool {
	return []tool.Tool{SearchPaper(), FindFigures()}
}

// ── search_paper ─────────────────────────────────────────────

type searchInput struct {
	Query string `json:"query" jsonschema:"description=要检索的问题或关键词,用自然语言描述;含指代时先改写成独立 query,required"`
}

type searchHit struct {
	Content string  `json:"content" jsonschema:"description=命中的文献片段正文"`
	Source  string  `json:"source" jsonschema:"description=出处:文件名、页码、片段序号与库类型,引用时务必带上"`
	Score   float64 `json:"score,omitempty" jsonschema:"description=相关度,越大越相关"`
}

type searchOutput struct {
	Hits []searchHit `json:"hits" jsonschema:"description=命中的文献片段列表;为空表示没检索到相关资料,如实告知不要编造"`
}

// SearchPaper 构建正文检索工具:身份从 ctx 取,论文范围用会话绑定的 paperID(空则跨可见库)。
func SearchPaper() tool.Tool {
	fn := func(ctx context.Context, in searchInput) (searchOutput, error) {
		owner := tenant.MustStudentID(ctx)
		docs, err := retrieval.RetrieveForPaper(ctx, in.Query, owner, core.PaperIDFrom(ctx))
		if err != nil {
			return searchOutput{}, fmt.Errorf("search_paper: 检索失败: %w", err)
		}
		docs = retrieval.DropImageDocs(docs) // 图块交给 find_figures,正文检索不混图说明
		zlog.Debug("search_paper 检索", "paper_id", core.PaperIDFrom(ctx), "query", in.Query, "hits", len(docs))
		retrieval.AddRefs(ctx, retrieval.References(docs))
		hits := make([]searchHit, 0, len(docs))
		for _, d := range docs {
			hits = append(hits, searchHit{
				Content: d.Content,
				Source:  retrieval.FormatReference(retrieval.ReferenceFromDocument(d)),
				Score:   d.Score,
			})
		}
		return searchOutput{Hits: hits}, nil
	}
	return function.NewFunctionTool(fn,
		function.WithName("search_paper"),
		function.WithDescription("在用户的论文知识库里做语义检索,返回带 source 出处的相关片段。可多次调用,每次用更聚焦或改写后的 query 检索不同侧面;回答务必依据返回片段并带上 source,检索不到就如实说明不要编造。"),
	)
}

// ── find_figures ─────────────────────────────────────────────

type figuresInput struct {
	Query string `json:"query" jsonschema:"description=要找的图表主题/对象,如某指标趋势、某数据集对比、某流程图,required"`
}

type figureHit struct {
	Description string  `json:"description" jsonschema:"description=图表内容说明(caption 与离线生成的图表描述)"`
	Figure      string  `json:"figure" jsonschema:"description=插图占位,形如 figure://文件名;要在正文引用该图时用 Markdown ![说明](该值) 原样插入,文件名不可改写"`
	Score       float64 `json:"score,omitempty" jsonschema:"description=相关度,越大越相关"`
}

type figuresOutput struct {
	Figures []figureHit `json:"figures" jsonschema:"description=相关的论文插图/表格列表;为空表示没有相关图表,正常作答不必插图"`
}

// FindFigures 构建图表检索工具(文本化):只回说明与 figure 占位,不传图片字节。
// 模型据说明判断是否插图,用 figure://文件名 占位由前端解析成取图 URL。
func FindFigures() tool.Tool {
	fn := func(ctx context.Context, in figuresInput) (figuresOutput, error) {
		owner := tenant.MustStudentID(ctx)
		docs, err := retrieval.RetrieveImagesForPaper(ctx, in.Query, owner, core.PaperIDFrom(ctx))
		if err != nil {
			return figuresOutput{}, fmt.Errorf("find_figures: 图块检索失败: %w", err)
		}
		zlog.Debug("find_figures 检索", "paper_id", core.PaperIDFrom(ctx), "query", in.Query, "hits", len(docs))
		retrieval.AddRefs(ctx, retrieval.References(docs))
		figs := make([]figureHit, 0, len(docs))
		for _, d := range docs {
			name := filepath.Base(retrieval.MetaString(d, constant.MilvusFieldImgURI))
			if name == "" {
				continue
			}
			figs = append(figs, figureHit{
				Description: d.Content,
				Figure:      "figure://" + name,
				Score:       d.Score,
			})
		}
		return figuresOutput{Figures: figs}, nil
	}
	return function.NewFunctionTool(fn,
		function.WithName("find_figures"),
		function.WithDescription("检索与问题相关的论文图表(架构图/流程图/结果曲线/表格原图等),返回其说明与 figure 占位。返回了合适的图就用 Markdown ![简短说明](figure://文件名) 把图插入正文对应位置,文件名只能用返回值里的不要编造;确实没有相关图时才不插。表格的可检索文本通常由 search_paper 返回,需要展示原表或版面时再引用这里的表格原图。"),
	)
}
