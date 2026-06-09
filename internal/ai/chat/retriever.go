package chat

import (
	"context"
	"fmt"
	"strings"

	trpcdocument "trpc.group/trpc-go/trpc-agent-go/knowledge/document"
	trpcreranker "trpc.group/trpc-go/trpc-agent-go/knowledge/reranker"

	"GopherPaper/internal/knowledge"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// reranker 是与用户无关的 cross-encoder 精排器,由 main 启动期建好经 Init 注入;nil 时退化为纯向量召回。
var reranker trpcreranker.Reranker

// Init 校验 trpc 知识库已就绪(InitTRPCStore 须在 main 启动期先调),并注入 rerank 精排器。
func Init(_ context.Context, rr trpcreranker.Reranker) error {
	if !knowledge.TRPCReady() {
		return fmt.Errorf("chat: trpc 知识库未初始化")
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

// RetrieveImagesForPaper 单独一轮只检索图块,按 score 阈值过滤后取前 TopKImages 张,
// 用于带图问答:不与正文同池竞争,避免相关图被正文块挤出 topK;无相关图时返回空。
func RetrieveImagesForPaper(ctx context.Context, query, ownerID, docID string) ([]*Doc, error) {
	candidateK := constant.TopKImages
	if reranker != nil {
		candidateK = constant.RecallTopKImages
	}
	res, err := knowledge.SearchImagesTRPC(ctx, query, ownerID, strings.TrimSpace(docID), candidateK)
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
			"img_uri", metaString(d, constant.MilvusFieldImgURI),
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

// dropImageDocs 从召回结果里剔除图块,使正文上下文不含图说明(图块由 RetrieveImagesForPaper 专管),
// 避免图块同时出现在两路造成重复。public 库块无 block_type 字段,不受影响。
func dropImageDocs(docs []*Doc) []*Doc {
	out := docs[:0]
	for _, d := range docs {
		if metaString(d, constant.MilvusFieldBlockType) == constant.BlockTypeImage {
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
	res, err := knowledge.SearchTRPC(ctx, query, ownerID, docID, candidateK)
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
		docs = append(docs, &Doc{
			ID:       r.Document.ID,
			Content:  r.Document.Content,
			MetaData: r.Document.Metadata,
			Score:    r.Score,
		})
	}
	return rerankDocs(ctx, query, docs, constant.TopKKnowledge), nil
}
