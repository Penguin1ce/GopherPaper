package chat_pipeline

import (
	"context"
	"fmt"
	"strings"

	"GopherPaper/internal/knowledge"
	"GopherPaper/pkg/constant"
)

// Init 校验 trpc 知识库已就绪(InitTRPCStore 须在 main 启动期先调)。
func Init(_ context.Context) error {
	if !knowledge.TRPCReady() {
		return fmt.Errorf("chat_pipeline: trpc 知识库未初始化")
	}
	return nil
}

// RetrieveVisible 按多租户可见性检索:科研基础库全员可见,私有论文库仅本人可见。
func RetrieveVisible(ctx context.Context, query, ownerID string) ([]*Doc, error) {
	return search(ctx, query, ownerID, "")
}

// RetrieveForPaper 围绕某篇论文检索:docID 非空时限定到该论文,否则回退到 owner+public。
func RetrieveForPaper(ctx context.Context, query, ownerID, docID string) ([]*Doc, error) {
	return search(ctx, query, ownerID, strings.TrimSpace(docID))
}

// search 走 trpc vectorstore 检索,结果收成本包 Doc 作召回数据容器。
func search(ctx context.Context, query, ownerID, docID string) ([]*Doc, error) {
	res, err := knowledge.SearchTRPC(ctx, query, ownerID, docID, constant.TopKKnowledge)
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
	return docs, nil
}
