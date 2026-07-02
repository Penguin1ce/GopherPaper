// Package retrieval 是 RAG 的检索与出处召回层:连 Milvus 召回、两阶段 rerank 精排、
// 出处格式化。独立成叶子包(只依赖 knowledge,不碰 agentrt/toolkit),
// 供 ai/chat、ai/report 的生成链路与 toolkit 的论文检索工具共用,避免依赖环。
package retrieval

import (
	"context"
	"fmt"
	"sort"
	"strings"

	trpcdocument "trpc.group/trpc-go/trpc-agent-go/knowledge/document"
	trpcreranker "trpc.group/trpc-go/trpc-agent-go/knowledge/reranker"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"

	"GopherPaper/internal/knowledge"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// reranker 是与用户无关的 cross-encoder 精排器,由 main 启动期建好经 Init 注入;nil 时退化为纯向量召回。
var reranker trpcreranker.Reranker

var (
	searchKnowledge       = knowledge.Search
	queryPageKnowledge    = knowledge.QueryPageRange
	searchImagesKnowledge = knowledge.SearchImages
)

// Init 校验 trpc 知识库已就绪(knowledge.Init 须在 main 启动期先调),并注入 rerank 精排器。
func Init(_ context.Context, rr trpcreranker.Reranker) error {
	if !knowledge.Ready() {
		return fmt.Errorf("retrieval: trpc 知识库未初始化")
	}
	reranker = rr
	return nil
}

// rerankDocs 把向量召回的候选块经 cross-encoder 按与 query 的相关性精排,截取前 topN。
// reranker 为 nil 或候选不足时直接按原序截断,精排失败则退化为向量序,均不阻断问答。
func rerankDocs(ctx context.Context, query string, docs []*Doc, topN int) []*Doc {
	if reranker == nil || len(docs) <= 1 {
		return truncateDocs(docs, topN)
	}
	results := make([]*trpcreranker.Result, len(docs))
	for i, d := range docs {
		results[i] = &trpcreranker.Result{
			Document: &trpcdocument.Document{ID: d.ID, Content: d.Content, Metadata: d.MetaData},
			Score:    d.Score,
		}
	}
	out, err := reranker.Rerank(ctx, &trpcreranker.Query{Text: query, FinalQuery: query}, results)
	if err != nil {
		zlog.Error("rerank 精排失败,退化为向量序", "query", query, "err", err)
		return truncateDocs(docs, topN)
	}
	reranked := make([]*Doc, 0, len(out))
	for _, r := range out {
		if r == nil || r.Document == nil {
			continue
		}
		reranked = append(reranked, &Doc{
			ID:       r.Document.ID,
			Content:  r.Document.Content,
			MetaData: r.Document.Metadata,
			Score:    r.Score,
		})
	}
	return truncateDocs(reranked, topN)
}

// truncateDocs 截取前 n 个,n<=0 或不足时原样返回。
func truncateDocs(docs []*Doc, n int) []*Doc {
	if n > 0 && len(docs) > n {
		return docs[:n]
	}
	return docs
}

// RetrieveVisible 按多租户可见性检索:科研基础库全员可见,私有论文库仅本人可见。
func RetrieveVisible(ctx context.Context, query, ownerID string) ([]*Doc, error) {
	return search(ctx, query, ownerID, "")
}

// RetrieveForPaper 围绕某篇论文检索:docID 非空时限定到该论文,否则回退到 owner+public。
func RetrieveForPaper(ctx context.Context, query, ownerID, docID string) ([]*Doc, error) {
	return search(ctx, query, ownerID, strings.TrimSpace(docID))
}

// RetrievePageForPaper 围绕某篇论文的指定页取上下文,用于精读页局部问答。
// 页邻域走标量 Query 直接取块,不做向量化/rerank:当前页全部块按 chunk_index 保留,
// 邻页块在软预算内补齐;页邻域全空时才回退整篇论文向量检索并标 fallback_scope。
func RetrievePageForPaper(ctx context.Context, query, ownerID, docID string, pageNo int) ([]*Doc, error) {
	docID = strings.TrimSpace(docID)
	if docID == "" || pageNo <= 0 {
		return nil, nil
	}
	pageStart := pageNo - 1
	if pageStart < 1 {
		pageStart = 1
	}
	res, err := queryPageKnowledge(ctx, ownerID, docID, pageStart, pageNo+1, constant.MaodiePageQueryLimit)
	if err != nil {
		return nil, err
	}
	docs := pageContextDocs(docsFromResults(res), pageNo)
	if len(docs) > 0 {
		return docs, nil
	}

	fallbackDocs, err := search(ctx, query, ownerID, docID)
	if err != nil {
		return nil, err
	}
	MarkFallbackScope(fallbackDocs, "paper")
	return truncateDocs(fallbackDocs, constant.TopKKnowledge), nil
}

// pageContextDocs 把页邻域块整理成适合喂模型的阅读上下文:
// 当前页块完整保留并按 chunk_index 排序;邻页块按页距/文档顺序在软预算内补齐。
func pageContextDocs(docs []*Doc, pageNo int) []*Doc {
	current := make([]*Doc, 0, len(docs))
	var neighbors []*Doc
	for _, doc := range docs {
		if DocCoversPage(doc, pageNo) {
			current = append(current, doc)
		} else {
			neighbors = append(neighbors, doc)
		}
	}
	sort.SliceStable(current, func(i, j int) bool { return docOrderLess(current[i], current[j], pageNo) })
	sort.SliceStable(neighbors, func(i, j int) bool { return neighborOrderLess(neighbors[i], neighbors[j], pageNo) })

	out := make([]*Doc, 0, len(docs))
	used := 0
	for _, doc := range current {
		out = append(out, doc)
		used += len([]rune(doc.Content))
	}
	for _, doc := range neighbors {
		next := used + len([]rune(doc.Content))
		if used > 0 && next > constant.MaodiePageContextMaxRunes {
			continue
		}
		out = append(out, doc)
		used = next
	}
	return out
}

func docOrderLess(a, b *Doc, pageNo int) bool {
	ai, bi := metaInt64(a, constant.MilvusFieldChunkIndex), metaInt64(b, constant.MilvusFieldChunkIndex)
	if ai != bi {
		return ai < bi
	}
	ap, bp := metaInt64(a, constant.MilvusFieldPageNo), metaInt64(b, constant.MilvusFieldPageNo)
	if ap != bp {
		return ap < bp
	}
	return a.ID < b.ID
}

func neighborOrderLess(a, b *Doc, pageNo int) bool {
	ad, bd := pageDistance(a, pageNo), pageDistance(b, pageNo)
	if ad != bd {
		return ad < bd
	}
	return docOrderLess(a, b, pageNo)
}

func pageDistance(doc *Doc, pageNo int) int64 {
	start := metaInt64(doc, constant.MilvusFieldPageNo)
	if start == 0 {
		return 1<<62 - 1
	}
	diff := start - int64(pageNo)
	if diff < 0 {
		return -diff
	}
	return diff
}

// DocCoversPage 判断块是否覆盖指定页:单页块看 page_no,跨页块看 [page_no, page_end]。
func DocCoversPage(doc *Doc, pageNo int) bool {
	start := metaInt64(doc, constant.MilvusFieldPageNo)
	if start == int64(pageNo) {
		return true
	}
	end := metaInt64(doc, constant.MilvusFieldPageEnd)
	return start > 0 && start <= int64(pageNo) && end >= int64(pageNo)
}

func markFallbackScope(doc *Doc, scope string) {
	if doc == nil || scope == "" {
		return
	}
	meta := make(map[string]any, len(doc.MetaData)+1)
	for k, v := range doc.MetaData {
		meta[k] = v
	}
	meta[constant.MetaKeyFallbackScope] = scope
	doc.MetaData = meta
}

func MarkFallbackScope(docs []*Doc, scope string) {
	for _, doc := range docs {
		markFallbackScope(doc, scope)
	}
}

func HasFallbackScope(docs []*Doc, scope string) bool {
	for _, doc := range docs {
		if MetaString(doc, constant.MetaKeyFallbackScope) == scope {
			return true
		}
	}
	return false
}

func docsFromResults(res *vectorstore.SearchResult) []*Doc {
	if res == nil {
		return nil
	}
	docs := make([]*Doc, 0, len(res.Results))
	for _, r := range res.Results {
		if r == nil || r.Document == nil {
			continue
		}
		docs = append(docs, &Doc{
			ID:       r.Document.ID,
			Content:  r.Document.Content,
			MetaData: r.Document.Metadata,
			Score:    r.Score,
		})
	}
	return docs
}

// RetrieveImagesForPaper 单独一轮只检索图块,按 score 阈值过滤后取前 TopKImages 张,
// 用于带图问答:不与正文同池竞争,避免相关图被正文块挤出 topK;无相关图时返回空。
func RetrieveImagesForPaper(ctx context.Context, query, ownerID, docID string) ([]*Doc, error) {
	candidateK := constant.TopKImages
	if reranker != nil {
		candidateK = constant.RecallTopKImages
	}
	res, err := searchImagesKnowledge(ctx, query, ownerID, strings.TrimSpace(docID), candidateK)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	docs := make([]*Doc, 0, len(res.Results))
	for _, r := range res.Results {
		if r == nil || r.Document == nil {
			continue
		}
		d := &Doc{
			ID:       r.Document.ID,
			Content:  r.Document.Content,
			MetaData: r.Document.Metadata,
			Score:    r.Score,
		}
		kept := r.Score >= constant.ImageScoreThreshold
		// 调试:输出图块召回的文本/分数/是否过阈值,排查带图问答召回情况(需 log.level=debug)。
		zlog.Debug("图块召回",
			"query", query,
			"img_uri", MetaString(d, constant.MilvusFieldImgURI),
			"score", r.Score,
			"threshold", constant.ImageScoreThreshold,
			"kept", kept,
			"content", d.Content,
		)
		if kept {
			docs = append(docs, d)
		}
	}
	// 过向量阈值的候选再经 cross-encoder 精排,按 caption/VLM 描述与 query 相关性截到 TopKImages。
	return rerankDocs(ctx, query, docs, constant.TopKImages), nil
}

// DropImageDocs 从召回结果里剔除图块,使正文上下文不含图说明(图块由 RetrieveImagesForPaper 专管),
// 避免图块同时出现在两路造成重复。public 库块无 block_type 字段,不受影响。
func DropImageDocs(docs []*Doc) []*Doc {
	out := docs[:0]
	for _, d := range docs {
		if MetaString(d, constant.MilvusFieldBlockType) == constant.BlockTypeImage {
			continue
		}
		out = append(out, d)
	}
	return out
}

// search 走 trpc vectorstore 检索,结果收成本包 Doc 作召回数据容器。
// 两阶段:开启 rerank 时先扩大向量召回到 RecallTopK 候选,再经 cross-encoder 精排截到 TopKKnowledge。
func search(ctx context.Context, query, ownerID, docID string) ([]*Doc, error) {
	candidateK := constant.TopKKnowledge
	if reranker != nil {
		candidateK = constant.RecallTopK
	}
	res, err := searchKnowledge(ctx, query, ownerID, docID, candidateK)
	if err != nil {
		return nil, err
	}
	docs := docsFromResults(res)
	return rerankDocs(ctx, query, docs, constant.TopKKnowledge), nil
}
