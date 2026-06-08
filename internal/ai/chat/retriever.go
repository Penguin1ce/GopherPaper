package chat

import (
	"context"
	"fmt"
	"strings"

	"GopherPaper/internal/knowledge"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// Init 校验 trpc 知识库已就绪(InitTRPCStore 须在 main 启动期先调)。
func Init(_ context.Context) error {
	if !knowledge.TRPCReady() {
		return fmt.Errorf("chat: trpc 知识库未初始化")
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

// RetrieveImagesForPaper 单独一轮只检索图块,按 score 阈值过滤后取前 TopKImages 张,
// 用于带图问答:不与正文同池竞争,避免相关图被正文块挤出 topK;无相关图时返回空。
func RetrieveImagesForPaper(ctx context.Context, query, ownerID, docID string) ([]*Doc, error) {
	res, err := knowledge.SearchImagesTRPC(ctx, query, ownerID, strings.TrimSpace(docID), constant.TopKImages)
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
	return docs, nil
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
