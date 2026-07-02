// Package maodie 是精读页小耄耋 agent:围绕当前页/选段做轻量随读问答。
package maodie

import (
	"context"
	"fmt"
	"strings"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/retrieval"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

var (
	retrievePageForPaper = retrieval.RetrievePageForPaper
	retrieveForPaper     = retrieval.RetrieveForPaper
)

// Chat 用当前阅读上下文做单轮局部问答,不走意图分类和 agentic 工具循环。
func Chat(ctx context.Context, history []trpcmodel.Message, query string, rc core.ReaderContext) (*core.Reply, error) {
	owner := tenant.MustStudentID(ctx)
	paperID := core.PaperIDFrom(ctx)
	rc = normalizeReaderContext(rc)

	docs := retrieveLocalDocs(ctx, owner, paperID, query, history, rc)
	// 图块不剔除:caption 与 VLM 描述留在 context 作证据,命中图本体随消息发给多模态模型,
	// 图表页(正文块稀少)才不会退化成无证据可答。
	// 页级取块是按位置而非相关性,只附覆盖当前页的图,邻页图仅留文本,防模型被邻页图带偏。
	imgDocs := retrieval.ImageDocs(docs)
	if rc.PageNo > 0 {
		imgDocs = imageDocsOnPage(imgDocs, rc.PageNo)
	}
	if len(imgDocs) > constant.TopKImages {
		imgDocs = imgDocs[:constant.TopKImages]
	}
	images := retrieval.LoadImagePayloads(imgDocs)
	sources := referencesWithSelection(owner, paperID, rc, retrieval.References(docs))

	models, err := aimodel.ModelsForUser(owner)
	if err != nil {
		return nil, err
	}
	model := models.Chat
	modelCfg := models.ChatMC
	if len(images) == 0 {
		model = models.Maodie
		modelCfg = models.MaodieMC
	}
	// 插图指令经占位符放在证据区之后,让「优先解释选段」的回答要求保持在 prompt 末尾的强位置。
	sysPrompt := strings.NewReplacer(
		"{reader_context}", formatReaderContext(rc),
		"{context}", retrieval.FormatDocs(docs),
		"{figure_instruction}", retrieval.FigureInstruction(imgDocs),
	).Replace(constant.MaodiePrompt)
	msgs := make([]trpcmodel.Message, 0, len(history)+2)
	msgs = append(msgs, trpcmodel.NewSystemMessage(sysPrompt))
	msgs = append(msgs, history...)
	msgs = append(msgs, userMessage(modelQuery(query, rc), images))
	req := &trpcmodel.Request{Messages: msgs}
	if modelCfg.MaxTokens > 0 {
		req.MaxTokens = &modelCfg.MaxTokens
	}
	content, err := core.StreamText(ctx, model, req, func(delta string) {
		if emit := core.StreamFrom(ctx); emit != nil {
			emit(core.StreamEvent{Kind: constant.StreamEventDelta, Delta: delta})
		}
	})
	if err != nil {
		return nil, err
	}
	reply := &core.Reply{Intent: constant.IntentMaodie, Content: content}
	reply.Meta = map[string]any{"reader_context": rc}
	if len(sources) > 0 {
		reply.Meta["sources"] = sources
	}
	if retrieval.HasFallbackScope(docs, "paper") {
		reply.Meta[constant.MetaKeyFallbackScope] = "paper"
	}
	return reply, nil
}

// modelQuery 构造喂模型的当前轮问题:有选段时把选段原文并入 user 轮次紧贴问题,
// 「这个/这段」的指代绑定在注意力最高的位置;历史与落库仍是原 query,不受影响。
func modelQuery(query string, rc core.ReaderContext) string {
	if rc.SelectedText == "" {
		return query
	}
	var b strings.Builder
	if rc.PageNo > 0 {
		fmt.Fprintf(&b, "我在第 %d 页选中了以下原文,问题里的「这个/这段/这里」都指这个选段:\n", rc.PageNo)
	} else {
		b.WriteString("我选中了以下原文,问题里的「这个/这段/这里」都指这个选段:\n")
	}
	b.WriteString("\"\"\"\n")
	b.WriteString(rc.SelectedText)
	b.WriteString("\n\"\"\"\n\n问题: ")
	b.WriteString(query)
	return b.String()
}

// imageDocsOnPage 只保留覆盖指定页的图块。
func imageDocsOnPage(imgDocs []*retrieval.Doc, pageNo int) []*retrieval.Doc {
	out := make([]*retrieval.Doc, 0, len(imgDocs))
	for _, d := range imgDocs {
		if retrieval.DocCoversPage(d, pageNo) {
			out = append(out, d)
		}
	}
	return out
}

// userMessage 构造当前 user 轮次:无命中图退化纯文本,有图时文本与图片同放 ContentParts,
// 与 agentrt 的带图问答同构;chat 模型须支持视觉。
func userMessage(query string, images []retrieval.ImagePayload) trpcmodel.Message {
	if len(images) == 0 {
		return trpcmodel.NewUserMessage(query)
	}
	msg := trpcmodel.Message{Role: trpcmodel.RoleUser}
	q := query
	msg.ContentParts = append(msg.ContentParts, trpcmodel.ContentPart{Type: trpcmodel.ContentTypeText, Text: &q})
	for _, img := range images {
		msg.AddImageData(img.Data, "auto", img.Format)
	}
	return msg
}

func retrieveLocalDocs(ctx context.Context, owner, paperID, query string, history []trpcmodel.Message, rc core.ReaderContext) []*retrieval.Doc {
	searchQuery := localSearchQuery(query, history, rc.SelectedText)
	if rc.PageNo > 0 {
		docs, err := retrievePageForPaper(ctx, searchQuery, owner, paperID, rc.PageNo)
		if err != nil {
			zlog.Warn("小耄耋当前页召回失败", "paper_id", paperID, "page_no", rc.PageNo, "err", err)
			return retrievePaperFallback(ctx, searchQuery, owner, paperID)
		}
		if len(docs) > 0 {
			return docs
		}
		return nil
	}
	docs, err := retrieveForPaper(ctx, searchQuery, owner, paperID)
	if err != nil {
		zlog.Warn("小耄耋当前论文召回失败", "paper_id", paperID, "err", err)
		return nil
	}
	return docs
}

func retrievePaperFallback(ctx context.Context, query, owner, paperID string) []*retrieval.Doc {
	docs, err := retrieveForPaper(ctx, query, owner, paperID)
	if err != nil {
		zlog.Warn("小耄耋全文补充召回失败", "paper_id", paperID, "err", err)
		return nil
	}
	retrieval.MarkFallbackScope(docs, "paper")
	return docs
}

func localSearchQuery(query string, history []trpcmodel.Message, selectedText string) string {
	searchQuery := strings.TrimSpace(query)
	if isShortFollowupQuery(searchQuery) {
		if prev := lastUserQuery(history); prev != "" {
			searchQuery = strings.TrimSpace(trimRunes(prev, constant.MaodieFollowupHistoryMaxRunes) + "\n" + searchQuery)
		}
	}
	selectedText = strings.TrimSpace(selectedText)
	if selectedText != "" {
		searchQuery = strings.TrimSpace(searchQuery + "\n" + selectedText)
	}
	if searchQuery == "" {
		searchQuery = selectedText
	}
	return searchQuery
}

func isShortFollowupQuery(query string) bool {
	return query != "" && len([]rune(query)) <= constant.MaodieFollowupQueryMaxRunes
}

func lastUserQuery(history []trpcmodel.Message) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role != trpcmodel.RoleUser {
			continue
		}
		if q := strings.TrimSpace(history[i].Content); q != "" {
			return q
		}
	}
	return ""
}

func normalizeReaderContext(rc core.ReaderContext) core.ReaderContext {
	rc.Scope = strings.TrimSpace(strings.ToLower(rc.Scope))
	if rc.Scope == "" {
		if strings.TrimSpace(rc.SelectedText) != "" {
			rc.Scope = "selection"
		} else {
			rc.Scope = "page"
		}
	}
	rc.SelectedText = trimRunes(strings.TrimSpace(rc.SelectedText), constant.MaxReaderContextRunes)
	if rc.PageNo < 0 {
		rc.PageNo = 0
	}
	switch rc.Scope {
	case "selection":
		if rc.SelectedText == "" {
			rc.Scope = "page"
		}
	case "paper":
		rc.PageNo = 0
		rc.SelectedText = ""
	case "page":
		rc.SelectedText = ""
	default:
		rc.Scope = "page"
		rc.SelectedText = ""
	}
	return rc
}

func formatReaderContext(rc core.ReaderContext) string {
	var lines []string
	switch rc.Scope {
	case "selection":
		lines = append(lines, "作用域: 用户选中的原文片段")
	case "page":
		lines = append(lines, "作用域: 当前页")
	case "paper":
		lines = append(lines, "作用域: 当前论文全文")
	default:
		lines = append(lines, "作用域: "+rc.Scope)
	}
	if rc.PageNo > 0 {
		lines = append(lines, fmt.Sprintf("当前页: 第 %d 页", rc.PageNo))
	}
	if rc.SelectedText != "" {
		// 选段全文已并入当前 user 轮次(见 modelQuery),此处只留指引避免重复占上下文。
		lines = append(lines, "选段原文: 已随本轮用户消息给出,「这个/这段」即指该选段")
	}
	if len(lines) == 0 {
		return "未提供局部阅读上下文"
	}
	return strings.Join(lines, "\n")
}

func referencesWithSelection(owner, paperID string, rc core.ReaderContext, refs []retrieval.Reference) []retrieval.Reference {
	if rc.SelectedText == "" || paperID == "" || rc.PageNo <= 0 {
		return refs
	}
	out := make([]retrieval.Reference, 0, len(refs)+1)
	selectionRef := retrieval.Reference{
		ID:        fmt.Sprintf("selection:%s:%d", paperID, rc.PageNo),
		Scope:     constant.KnowledgeScopePrivate,
		StudentID: owner,
		DocID:     paperID,
		PageNo:    int64(rc.PageNo),
		BlockType: "selection",
	}
	selectionRef.CitationTag = retrieval.FormatCitationTag(selectionRef)
	out = append(out, selectionRef)
	return append(out, refs...)
}

func trimRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
