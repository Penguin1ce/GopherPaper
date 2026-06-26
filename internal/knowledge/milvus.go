// milvus.go 封装 trpc Milvus vectorstore 与 trpc embedder。
// trpc vectorstore 使用固定 schema，项目标量字段统一写入 metadata JSON。
// 多租户过滤经 searchfilter 表达，BM25 全文检索需 Milvus 2.5 及以上版本。
package knowledge

import (
	"context"
	"fmt"
	"maps"
	"strings"

	mventity "github.com/milvus-io/milvus/client/v2/entity"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/document"
	trpcembedder "trpc.group/trpc-go/trpc-agent-go/knowledge/embedder"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/searchfilter"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"
	mvstore "trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore/milvus"

	"GopherPaper/internal/config"
	"GopherPaper/pkg/constant"
)

var (
	trpcStore *mvstore.VectorStore
	trpcEmb   trpcembedder.Embedder
	trpcDim   int
)

// Init 用 trpc vectorstore 与 embedder 初始化知识库 collection。
func Init(ctx context.Context, mc config.MilvusConfig, collection string, emb trpcembedder.Embedder, dim int) error {
	if emb == nil {
		return fmt.Errorf("knowledge: trpc embedder 不能为空")
	}
	if dim <= 0 {
		return fmt.Errorf("knowledge: 向量维度必须大于 0")
	}
	vs, err := mvstore.New(ctx,
		mvstore.WithAddress(mc.Address),
		mvstore.WithUsername(mc.Username),
		mvstore.WithPassword(mc.Password),
		mvstore.WithCollectionName(collection),
		mvstore.WithDimension(dim),
		mvstore.WithMetricType(mventity.COSINE),
	)
	if err != nil {
		return fmt.Errorf("knowledge: 初始化 trpc vectorstore 失败: %w", err)
	}
	trpcStore = vs
	trpcEmb = emb
	trpcDim = dim
	return nil
}

// AddChunk 把一个 chunk 规整后写入 trpc vectorstore。
func AddChunk(ctx context.Context, chunk Chunk) error {
	if err := normalizeChunk(&chunk); err != nil {
		return err
	}
	return addChunk(ctx, chunk)
}

// addChunk 把一个已规整的 chunk 写入 trpc vectorstore:标量字段落 metadata,向量经 trpc embedder 现算。
func addChunk(ctx context.Context, chunk Chunk) error {
	if trpcStore == nil || trpcEmb == nil {
		return fmt.Errorf("knowledge: trpc store 未初始化")
	}
	vec, err := trpcEmb.GetEmbedding(ctx, chunk.Content)
	if err != nil {
		return fmt.Errorf("knowledge: trpc 向量化失败: %w", err)
	}
	doc := &document.Document{
		ID:       chunk.ID,
		Content:  chunk.Content,
		Metadata: chunkMetadata(chunk),
	}
	if err := trpcStore.Add(ctx, doc, vec); err != nil {
		return fmt.Errorf("knowledge: trpc 写入失败: %w", err)
	}
	return nil
}

// Ready 报告 trpc 知识库是否已初始化。
func Ready() bool { return trpcStore != nil && trpcEmb != nil }

// DeletePaperChunks removes all private chunks for one uploaded paper.
func DeletePaperChunks(ctx context.Context, ownerID, paperID string) error {
	if trpcStore == nil {
		return fmt.Errorf("knowledge: trpc store 未初始化")
	}
	filter := map[string]any{
		constant.MilvusFieldKnowledgeScope: string(constant.KnowledgeScopePrivate),
		constant.MilvusFieldStudentID:      ownerID,
		constant.MilvusFieldDocID:          paperID,
	}
	if err := trpcStore.DeleteByFilter(ctx, vectorstore.WithDeleteFilter(filter)); err != nil {
		return fmt.Errorf("knowledge: 删除论文 chunks 失败: %w", err)
	}
	return nil
}

// Embed 用同一 Qwen3-Embedding embedder 把任意文本向量化,供知识图谱算论文语义相似度复用,
// 与入库 chunk 走同一向量空间。embedder 未就绪返回错误。
func Embed(ctx context.Context, text string) ([]float64, error) {
	if trpcEmb == nil {
		return nil, fmt.Errorf("knowledge: embedder 未初始化")
	}
	return trpcEmb.GetEmbedding(ctx, text)
}

// Close 关闭 trpc vectorstore 的 Milvus 连接,在服务关停时调用。
func Close() error {
	if trpcStore == nil {
		return nil
	}
	return trpcStore.Close()
}

// UpsertChunks 批量写入 chunk(逐条 Add,只规整一次),返回写入的 id。
func UpsertChunks(ctx context.Context, chunks []Chunk) ([]string, error) {
	ids := make([]string, 0, len(chunks))
	for i := range chunks {
		if err := normalizeChunk(&chunks[i]); err != nil {
			return ids, err
		}
		if err := addChunk(ctx, chunks[i]); err != nil {
			return ids, err
		}
		ids = append(ids, chunks[i].ID)
	}
	return ids, nil
}

// Search 按多租户可见性混合检索:科研基础库全员可见,私有库仅本人可见;docID 非空时限定到该论文。
func Search(ctx context.Context, query, ownerID, docID string, topK int) (*vectorstore.SearchResult, error) {
	return searchWithFilter(ctx, query, scopeCondition(ownerID, docID), topK)
}

// SearchImages 只检索图块(block_type==image),用于问答时单独一轮带图召回,
// 不与正文同池竞争。可见性过滤同 Search。
func SearchImages(ctx context.Context, query, ownerID, docID string, topK int) (*vectorstore.SearchResult, error) {
	filter := searchfilter.And(
		scopeCondition(ownerID, docID),
		searchfilter.Equal(metadataPrefix+constant.MilvusFieldBlockType, constant.BlockTypeImage),
	)
	return searchWithFilter(ctx, query, filter, topK)
}

// searchWithFilter 用给定过滤条件做一次混合检索,统一处理向量化与空结果。
func searchWithFilter(ctx context.Context, query string, filter *searchfilter.UniversalFilterCondition, topK int) (*vectorstore.SearchResult, error) {
	if trpcStore == nil || trpcEmb == nil {
		return nil, fmt.Errorf("knowledge: trpc store 未初始化")
	}
	query = strings.TrimSpace(query)
	vec, err := trpcEmb.GetEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("knowledge: trpc 查询向量化失败: %w", err)
	}
	res, err := trpcStore.Search(ctx, &vectorstore.SearchQuery{
		Query:      query,
		Vector:     vec,
		Limit:      topK,
		SearchMode: vectorstore.SearchModeHybrid,
		Filter:     &vectorstore.SearchFilter{FilterCondition: filter},
	})
	if err != nil {
		if strings.Contains(err.Error(), "no results found") {
			return &vectorstore.SearchResult{}, nil
		}
		return nil, err
	}
	return res, nil
}

// chunkMetadata 把 chunk 的标量字段汇成 metadata(trpc schema 无独立标量列,全部落此)。
func chunkMetadata(chunk Chunk) map[string]any {
	meta := map[string]any{
		constant.MilvusFieldKnowledgeScope: string(chunk.Scope),
		constant.MilvusFieldStudentID:      chunk.OwnerID,
		constant.MilvusFieldDocID:          chunk.DocID,
		constant.MilvusFieldSourceFile:     chunk.SourceFile,
		constant.MilvusFieldSourceURI:      chunk.SourceURI,
		constant.MilvusFieldPageNo:         chunk.PageNo,
		constant.MilvusFieldChunkIndex:     chunk.ChunkIndex,
		constant.MilvusFieldCreatedAt:      chunk.CreatedAt,
	}
	maps.Copy(meta, chunk.Metadata)
	return meta
}

// metadataPrefix 对应 trpc source.MetadataFieldPrefix:标量字段落在 metadata JSON,
// 过滤字段须带此前缀,condition_converter 才会转成 Milvus 的 metadata["xxx"] 访问路径。
const metadataPrefix = "metadata."

// scopeCondition 构造多租户过滤:(scope==public) or (scope==private and student_id==owner),
// docID 非空时再 and doc_id==docID。
func scopeCondition(ownerID, docID string) *searchfilter.UniversalFilterCondition {
	pub := searchfilter.Equal(metadataPrefix+constant.MilvusFieldKnowledgeScope, string(constant.KnowledgeScopePublic))
	if ownerID == "" {
		return pub
	}
	privTerms := []*searchfilter.UniversalFilterCondition{
		searchfilter.Equal(metadataPrefix+constant.MilvusFieldKnowledgeScope, string(constant.KnowledgeScopePrivate)),
		searchfilter.Equal(metadataPrefix+constant.MilvusFieldStudentID, ownerID),
	}
	if docID != "" {
		privTerms = append(privTerms, searchfilter.Equal(metadataPrefix+constant.MilvusFieldDocID, docID))
		return searchfilter.And(privTerms...)
	}
	return searchfilter.Or(pub, searchfilter.And(privTerms...))
}
