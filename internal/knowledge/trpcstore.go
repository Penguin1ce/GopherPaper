// trpcstore.go 是 Step 2 方案Y(全 trpc Knowledge)的封装层。
//
// 用 trpc 的 milvus vectorstore(底层走新版 SDK milvus-io/milvus/client/v2)+ trpc embedder。
// trpc vectorstore 是固定 schema(id/name/content/sparse/vector/metadata/created_at/
// updated_at + BM25),故本项目的标量字段(scope/student_id/doc_id/...)全部落入 metadata
// JSON,多租户过滤经 searchfilter 在 metadata 上表达。BM25 全文检索需 Milvus 2.5+。
package knowledge

import (
	"context"
	"fmt"
	"maps"

	mventity "github.com/milvus-io/milvus/client/v2/entity"
	trpcembedder "trpc.group/trpc-go/trpc-agent-go/knowledge/embedder"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/document"
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

// InitTRPCStore 用 trpc vectorstore + embedder 初始化方案Y 知识库,走独立 collection。
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

// AddChunkTRPC 把一个 chunk 写入 trpc vectorstore:标量字段落 metadata,向量经 trpc embedder 现算。
func AddChunkTRPC(ctx context.Context, chunk Chunk) error {
	if trpcStore == nil || trpcEmb == nil {
		return fmt.Errorf("knowledge: trpc store 未初始化")
	}
	if err := normalizeChunk(&chunk); err != nil {
		return err
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

// UpsertChunksTRPC 批量写入 chunk(逐条 Add),返回写入的 id。
func UpsertChunksTRPC(ctx context.Context, chunks []Chunk) ([]string, error) {
	ids := make([]string, 0, len(chunks))
	for i := range chunks {
		if err := normalizeChunk(&chunks[i]); err != nil {
			return ids, err
		}
		if err := AddChunkTRPC(ctx, chunks[i]); err != nil {
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
	return trpcStore.Search(ctx, &vectorstore.SearchQuery{
		Vector:     vec,
		Limit:      topK,
		SearchMode: vectorstore.SearchModeVector,
		Filter:     &vectorstore.SearchFilter{FilterCondition: scopeCondition(ownerID, docID)},
	})
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
