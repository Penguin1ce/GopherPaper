// toolkit paper.go 是论文知识库相关的 function tool,挂给小云雀,贴合本系统的论文库定位:
//   - list_my_papers:列出用户上传的论文(标题/状态/id),让 agent 先定位再检索
//   - search_my_papers:在用户可见库做语义检索,带 paper_id 时限定到某一篇,否则跨全库
//
// 检索走 retrieval 叶子包,论文列表直接走 paperdao(只碰 DB,均不依赖 toolkit,无依赖环)。
package toolkit

import (
	"context"
	"fmt"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"

	"GopherPaper/internal/ai/retrieval"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/tenant"
)

// ── list_my_papers ───────────────────────────────────────────

type listPapersInput struct {
	Keyword string `json:"keyword,omitempty" jsonschema:"description=按标题/文件名模糊筛选的关键词,留空则列出全部"`
}

type paperBrief struct {
	PaperID    string `json:"paper_id" jsonschema:"description=论文 id,传给 search_my_papers 的 paper_id 即可限定检索到这一篇"`
	Title      string `json:"title" jsonschema:"description=论文标题,无标题时回退文件名"`
	Status     string `json:"status" jsonschema:"description=解析状态,只有 ready/indexed 的论文已入库可被检索,其余仍在处理"`
	UploadedAt string `json:"uploaded_at,omitempty" jsonschema:"description=上传日期"`
}

type listPapersOutput struct {
	Papers []paperBrief `json:"papers" jsonschema:"description=用户的论文列表;为空表示该用户还没传过论文"`
}

// newListPapersTool 构建论文列表工具,身份从 ctx 的 tenant 取,只列本人的论文。
func newListPapersTool() tool.Tool {
	fn := func(ctx context.Context, in listPapersInput) (listPapersOutput, error) {
		owner := tenant.MustStudentID(ctx)
		papers, err := listPapers(ctx, owner, strings.TrimSpace(in.Keyword))
		if err != nil {
			return listPapersOutput{}, fmt.Errorf("list_my_papers: 查询论文列表失败: %w", err)
		}
		out := make([]paperBrief, 0, len(papers))
		for _, p := range papers {
			out = append(out, paperBrief{
				PaperID:    p.id,
				Title:      p.title,
				Status:     p.status,
				UploadedAt: p.uploadedAt,
			})
		}
		return listPapersOutput{Papers: out}, nil
	}
	return function.NewFunctionTool(fn,
		function.WithName("list_my_papers"),
		function.WithDescription("列出用户已上传的论文(标题、解析状态与 id)。当用户问'我传过哪些论文',或需要先定位某一篇再检索其内容时调用;拿到的 paper_id 可传给 search_my_papers 限定检索到那一篇。"),
	)
}

// paperBriefSource 是从 dao 取出的论文摘要的中间载体,避免在 fn 里直接耦合 model 字段。
type paperBriefSource struct {
	id         string
	title      string
	status     string
	uploadedAt string
}

// listPapers 取某用户的论文,keyword 非空时按标题/文件名模糊匹配。
func listPapers(ctx context.Context, owner, keyword string) ([]paperBriefSource, error) {
	ps, err := paperdao.List(ctx, owner)
	if keyword != "" {
		ps, err = paperdao.Search(ctx, owner, keyword)
	}
	if err != nil {
		return nil, err
	}
	out := make([]paperBriefSource, 0, len(ps))
	for _, p := range ps {
		title := p.Title
		if strings.TrimSpace(title) == "" {
			title = p.FileName
		}
		out = append(out, paperBriefSource{
			id:         p.ID,
			title:      title,
			status:     string(p.Status),
			uploadedAt: p.CreatedAt.Format("2006-01-02"),
		})
	}
	return out, nil
}

// ── search_my_papers ─────────────────────────────────────────

type paperSearchInput struct {
	Query   string `json:"query" jsonschema:"description=要在科研知识库里检索的问题或关键词,用自然语言描述,required"`
	PaperID string `json:"paper_id,omitempty" jsonschema:"description=限定检索某一篇论文时填其 id(从 list_my_papers 取);留空则跨用户全部可见库(本人私有论文+公共科研基础库)检索"`
}

type paperHit struct {
	Content string  `json:"content" jsonschema:"description=命中的文献片段正文"`
	Source  string  `json:"source" jsonschema:"description=出处:文件名、页码、片段序号与库类型(public 科研基础库 / private 用户论文)"`
	Score   float64 `json:"score,omitempty" jsonschema:"description=相关度,越大越相关"`
}

type paperSearchOutput struct {
	Hits []paperHit `json:"hits" jsonschema:"description=命中的文献片段列表,引用时务必带上对应 source 出处;为空表示库里没有相关资料,如实告知用户不要编造"`
}

// newPaperSearchTool 构建论文库检索工具。用户身份从 ctx 的 tenant 取;
// paper_id 非空限定到该论文(RetrieveForPaper),否则走多租户可见性全库检索(RetrieveVisible)。
func newPaperSearchTool() tool.Tool {
	fn := func(ctx context.Context, in paperSearchInput) (paperSearchOutput, error) {
		owner := tenant.MustStudentID(ctx)
		var (
			docs []*retrieval.Doc
			err  error
		)
		if pid := strings.TrimSpace(in.PaperID); pid != "" {
			docs, err = retrieval.RetrieveForPaper(ctx, in.Query, owner, pid)
		} else {
			docs, err = retrieval.RetrieveVisible(ctx, in.Query, owner)
		}
		if err != nil {
			return paperSearchOutput{}, fmt.Errorf("search_my_papers: 检索失败: %w", err)
		}
		hits := make([]paperHit, 0, len(docs))
		for _, d := range docs {
			ref := retrieval.ReferenceFromDocument(d)
			hits = append(hits, paperHit{
				Content: d.Content,
				Source:  retrieval.FormatReference(ref),
				Score:   d.Score,
			})
		}
		return paperSearchOutput{Hits: hits}, nil
	}
	return function.NewFunctionTool(fn,
		function.WithName("search_my_papers"),
		function.WithDescription("在用户的科研知识库里做语义检索。问某一篇论文的内容时先用 list_my_papers 拿到 paper_id 再带上限定到该篇;泛泛找'我传过的论文里有没有讲过 X'则不填 paper_id 跨全库检索。回答附上返回的 source 出处,库里没有就如实说明不要编造。"),
	)
}
