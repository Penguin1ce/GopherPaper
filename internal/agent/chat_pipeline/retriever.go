package chat_pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mvretriever "github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"

	"GopherPaper/internal/knowledge"
	"GopherPaper/pkg/constant"
)

var outputFields = []string{
	constant.MilvusFieldID,
	constant.MilvusFieldContent,
	constant.MilvusFieldMetadata,
	constant.MilvusFieldKnowledgeScope,
	constant.MilvusFieldStudentID,
	constant.MilvusFieldDocID,
	constant.MilvusFieldSourceFile,
	constant.MilvusFieldSourceURI,
	constant.MilvusFieldPageNo,
	constant.MilvusFieldChunkIndex,
	constant.MilvusFieldCreatedAt,
}

var retr retriever.Retriever

// Init 用知识库已就绪的 Milvus 句柄构建检索器，须在 knowledge.Init 之后调用。
func Init(ctx context.Context) error {
	if knowledge.Client() == nil || knowledge.Embedder() == nil {
		return fmt.Errorf("chat_pipeline: knowledge 未初始化")
	}
	r, err := newRetriever(ctx)
	if err != nil {
		return err
	}
	retr = r
	return nil
}

// RetrieveVisible 按多租户可见性检索：科研基础库全员可见，私有论文库仅本人可见。
func RetrieveVisible(ctx context.Context, query, ownerID string) ([]*schema.Document, error) {
	return retrieve(ctx, query, visibleFilter(ownerID))
}

// RetrieveForPaper 围绕某篇论文检索：docID 非空时限定到该论文，否则回退到 owner+public。
func RetrieveForPaper(ctx context.Context, query, ownerID, docID string) ([]*schema.Document, error) {
	if strings.TrimSpace(docID) == "" {
		return retrieve(ctx, query, visibleFilter(ownerID))
	}
	return retrieve(ctx, query, paperFilter(ownerID, docID))
}

func retrieve(ctx context.Context, query, filter string) ([]*schema.Document, error) {
	if retr == nil {
		return nil, fmt.Errorf("chat_pipeline: 检索器未初始化")
	}
	docs, err := retr.Retrieve(ctx, query, mvretriever.WithFilter(filter))
	if err != nil {
		if strings.Contains(err.Error(), "no results found") {
			return nil, nil
		}
		return nil, err
	}
	return docs, nil
}

func newRetriever(ctx context.Context) (retriever.Retriever, error) {
	return mvretriever.NewRetriever(ctx, &mvretriever.RetrieverConfig{
		Client:            knowledge.Client(),
		Collection:        knowledge.Collection(),
		VectorField:       constant.MilvusFieldVector,
		OutputFields:      outputFields,
		DocumentConverter: convertSearchResult,
		VectorConverter:   denseVectorConverter,
		MetricType:        entity.COSINE,
		TopK:              constant.TopKKnowledge,
		Embedding:         knowledge.Embedder(),
	})
}

func denseVectorConverter(_ context.Context, vectors [][]float64) ([]entity.Vector, error) {
	out := make([]entity.Vector, 0, len(vectors))
	for _, vector := range vectors {
		v, err := toFloat32(vector)
		if err != nil {
			return nil, err
		}
		out = append(out, entity.FloatVector(v))
	}
	return out, nil
}

func convertSearchResult(_ context.Context, result client.SearchResult) ([]*schema.Document, error) {
	n := result.IDs.Len()
	docs := make([]*schema.Document, 0, n)
	for i := 0; i < n; i++ {
		id, err := result.IDs.GetAsString(i)
		if err != nil {
			return nil, fmt.Errorf("chat_pipeline: 读取 id 失败: %w", err)
		}
		doc := &schema.Document{
			ID:      id,
			Content: columnString(result.Fields.GetColumn(constant.MilvusFieldContent), i),
			MetaData: map[string]any{
				constant.MilvusFieldKnowledgeScope: columnString(result.Fields.GetColumn(constant.MilvusFieldKnowledgeScope), i),
				constant.MilvusFieldStudentID:      columnString(result.Fields.GetColumn(constant.MilvusFieldStudentID), i),
				constant.MilvusFieldDocID:          columnString(result.Fields.GetColumn(constant.MilvusFieldDocID), i),
				constant.MilvusFieldSourceFile:     columnString(result.Fields.GetColumn(constant.MilvusFieldSourceFile), i),
				constant.MilvusFieldSourceURI:      columnString(result.Fields.GetColumn(constant.MilvusFieldSourceURI), i),
				constant.MilvusFieldPageNo:         columnInt64(result.Fields.GetColumn(constant.MilvusFieldPageNo), i),
				constant.MilvusFieldChunkIndex:     columnInt64(result.Fields.GetColumn(constant.MilvusFieldChunkIndex), i),
				constant.MilvusFieldCreatedAt:      columnInt64(result.Fields.GetColumn(constant.MilvusFieldCreatedAt), i),
			},
		}
		for k, v := range columnJSON(result.Fields.GetColumn(constant.MilvusFieldMetadata), i) {
			doc.MetaData[k] = v
		}
		if i < len(result.Scores) {
			doc.WithScore(float64(result.Scores[i]))
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func visibleFilter(ownerID string) string {
	publicFilter := fmt.Sprintf("%s == %s", constant.MilvusFieldKnowledgeScope, quoteExpr(string(constant.KnowledgeScopePublic)))
	if strings.TrimSpace(ownerID) == "" {
		return publicFilter
	}
	privateFilter := fmt.Sprintf("%s == %s and %s == %s",
		constant.MilvusFieldKnowledgeScope,
		quoteExpr(string(constant.KnowledgeScopePrivate)),
		constant.MilvusFieldStudentID,
		quoteExpr(ownerID),
	)
	return fmt.Sprintf("(%s) or (%s)",
		publicFilter,
		privateFilter,
	)
}

// paperFilter 限定到某用户的某篇私有论文。
func paperFilter(ownerID, docID string) string {
	return fmt.Sprintf("%s == %s and %s == %s and %s == %s",
		constant.MilvusFieldKnowledgeScope,
		quoteExpr(string(constant.KnowledgeScopePrivate)),
		constant.MilvusFieldStudentID,
		quoteExpr(ownerID),
		constant.MilvusFieldDocID,
		quoteExpr(docID),
	)
}

func toFloat32(vector []float64) ([]float32, error) {
	dim := knowledge.VectorDim()
	if len(vector) != dim {
		return nil, fmt.Errorf("chat_pipeline: 向量维度不匹配，期望 %d 实际 %d", dim, len(vector))
	}
	out := make([]float32, 0, len(vector))
	for _, v := range vector {
		out = append(out, float32(v))
	}
	return out, nil
}

func quoteExpr(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return `'` + s + `'`
}

func columnString(col entity.Column, idx int) string {
	if col == nil {
		return ""
	}
	v, err := col.GetAsString(idx)
	if err == nil {
		return v
	}
	raw, err := col.Get(idx)
	if err != nil || raw == nil {
		return ""
	}
	return fmt.Sprint(raw)
}

func columnInt64(col entity.Column, idx int) int64 {
	if col == nil {
		return 0
	}
	v, err := col.GetAsInt64(idx)
	if err == nil {
		return v
	}
	raw, err := col.Get(idx)
	if err != nil {
		return 0
	}
	switch x := raw.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	default:
		return 0
	}
}

func columnJSON(col entity.Column, idx int) map[string]any {
	out := map[string]any{}
	if col == nil {
		return out
	}
	raw, err := col.Get(idx)
	if err != nil || raw == nil {
		return out
	}
	bytes, ok := raw.([]byte)
	if !ok || len(bytes) == 0 {
		return out
	}
	_ = json.Unmarshal(bytes, &out)
	return out
}
