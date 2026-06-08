// trpcstore.go 封装 trpc Milvus vectorstore 与 trpc embedder。
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

// InitTRPCStore 用 trpc vectorstore 与 embedder 初始化知识库 collection。
func InitTRPCStore(ctx context.Context, mc config.MilvusConfig, collection string, emb trpcembedder.Embedder, dim int) error {
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

// AddChunkTRPC 把一个 chunk 规整后写入 trpc vectorstore。
func AddChunkTRPC(ctx context.Context, chunk Chunk) error {
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

// TRPCReady 报告 trpc 知识库是否已初始化。
func TRPCReady() bool { return trpcStore != nil && trpcEmb != nil }

// CloseTRPC 关闭 trpc vectorstore 的 Milvus 连接,在服务关停时调用。
func CloseTRPC() error {
	if trpcStore == nil {
		return nil
	}
	return trpcStore.Close()
}

// UpsertChunksTRPC 批量写入 chunk(逐条 Add,只规整一次),返回写入的 id。
func UpsertChunksTRPC(ctx context.Context, chunks []Chunk) ([]string, error) {
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

// SearchTRPC 按多租户可见性向量检索:科研基础库全员可见,私有库仅本人可见;docID 非空时限定到该论文。
func SearchTRPC(ctx context.Context, query, ownerID, docID string, topK int) (*vectorstore.SearchResult, error) {
	if trpcStore == nil || trpcEmb == nil {
		return nil, fmt.Errorf("knowledge: trpc store 未初始化")
	}
	vec, err := trpcEmb.GetEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("knowledge: trpc 查询向量化失败: %w", err)
	}
	res, err := trpcStore.Search(ctx, &vectorstore.SearchQuery{
		Vector:     vec,
		Limit:      topK,
		SearchMode: vectorstore.SearchModeVector,
		Filter:     &vectorstore.SearchFilter{FilterCondition: scopeCondition(ownerID, docID)},
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
