package knowledge

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/knowledge/document"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"

	"GopherPaper/pkg/constant"
)

func TestUpsertChunksDeletesExistingIDsBeforeInsert(t *testing.T) {
	restore := installTestKnowledgeStore()
	defer restore()

	ctx := context.Background()
	chunk := Chunk{
		Content:    "图神经网络用于论文引用预测",
		Scope:      constant.KnowledgeScopePrivate,
		OwnerID:    "userA",
		DocID:      "paperA",
		SourceFile: "paper.pdf",
		ChunkIndex: 1,
	}

	firstIDs, err := UpsertChunks(ctx, []Chunk{chunk})
	if err != nil {
		t.Fatalf("first UpsertChunks: %v", err)
	}
	secondIDs, err := UpsertChunks(ctx, []Chunk{chunk})
	if err != nil {
		t.Fatalf("second UpsertChunks should delete before insert: %v", err)
	}
	if !slices.Equal(firstIDs, secondIDs) {
		t.Fatalf("重复 upsert 返回 ID 不一致: first=%v second=%v", firstIDs, secondIDs)
	}

	store := trpcStore.(*insertOnlyVectorStore)
	if len(store.docs) != 1 {
		t.Fatalf("重复 upsert 后文档数=%d, want 1", len(store.docs))
	}
	if len(store.deleteIDs) != 2 {
		t.Fatalf("DeleteByFilter 调用次数=%d, want 2", len(store.deleteIDs))
	}
	if !slices.Equal(store.deleteIDs[1], firstIDs) {
		t.Fatalf("第二次删除 ID=%v, want %v", store.deleteIDs[1], firstIDs)
	}
}

func TestUpsertChunksDeduplicatesBatchByID(t *testing.T) {
	restore := installTestKnowledgeStore()
	defer restore()

	ids, err := UpsertChunks(context.Background(), []Chunk{
		{
			ID:      "same-id",
			Content: "旧内容",
			Scope:   constant.KnowledgeScopePrivate,
			OwnerID: "userA",
			DocID:   "paperA",
		},
		{
			ID:      "same-id",
			Content: "新内容",
			Scope:   constant.KnowledgeScopePrivate,
			OwnerID: "userA",
			DocID:   "paperA",
		},
	})
	if err != nil {
		t.Fatalf("UpsertChunks: %v", err)
	}
	if !slices.Equal(ids, []string{"same-id"}) {
		t.Fatalf("ids=%v, want [same-id]", ids)
	}

	store := trpcStore.(*insertOnlyVectorStore)
	doc := store.docs["same-id"]
	if doc == nil {
		t.Fatal("same-id 未写入")
	}
	if doc.Content != "新内容" {
		t.Fatalf("批内重复 ID 应以后者为准, got %q", doc.Content)
	}
	if len(store.addIDs) != 1 {
		t.Fatalf("Add 调用次数=%d, want 1", len(store.addIDs))
	}
	if len(store.deleteIDs) != 1 || !slices.Equal(store.deleteIDs[0], []string{"same-id"}) {
		t.Fatalf("删除 ID 记录=%v, want [[same-id]]", store.deleteIDs)
	}

	emb := trpcEmb.(*fixedEmbedder)
	if !slices.Equal(emb.texts, []string{"新内容"}) {
		t.Fatalf("向量化文本=%v, want [新内容]", emb.texts)
	}
}

func TestUpsertChunksEmptyDoesNotRequireStore(t *testing.T) {
	oldStore, oldEmb := trpcStore, trpcEmb
	trpcStore, trpcEmb = nil, nil
	defer func() {
		trpcStore, trpcEmb = oldStore, oldEmb
	}()

	ids, err := UpsertChunks(context.Background(), nil)
	if err != nil {
		t.Fatalf("empty UpsertChunks should be no-op: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("empty ids len=%d, want 0", len(ids))
	}
}

func installTestKnowledgeStore() func() {
	oldStore, oldEmb, oldDim := trpcStore, trpcEmb, trpcDim
	trpcStore = &insertOnlyVectorStore{docs: map[string]*document.Document{}}
	trpcEmb = &fixedEmbedder{}
	trpcDim = 3
	return func() {
		trpcStore, trpcEmb, trpcDim = oldStore, oldEmb, oldDim
	}
}

type fixedEmbedder struct {
	texts []string
}

func (e *fixedEmbedder) GetEmbedding(_ context.Context, text string) ([]float64, error) {
	e.texts = append(e.texts, text)
	return []float64{1, 2, 3}, nil
}

func (e *fixedEmbedder) GetEmbeddingWithUsage(ctx context.Context, text string) ([]float64, map[string]any, error) {
	vec, err := e.GetEmbedding(ctx, text)
	return vec, nil, err
}

func (e *fixedEmbedder) GetDimensions() int {
	return 3
}

type insertOnlyVectorStore struct {
	docs      map[string]*document.Document
	addIDs    []string
	deleteIDs [][]string
}

func (s *insertOnlyVectorStore) Add(_ context.Context, doc *document.Document, embedding []float64) error {
	if doc == nil {
		return fmt.Errorf("document required")
	}
	if _, exists := s.docs[doc.ID]; exists {
		return fmt.Errorf("duplicate primary key: %s", doc.ID)
	}
	if len(embedding) == 0 {
		return fmt.Errorf("embedding required")
	}
	s.docs[doc.ID] = doc
	s.addIDs = append(s.addIDs, doc.ID)
	return nil
}

func (s *insertOnlyVectorStore) Get(_ context.Context, id string) (*document.Document, []float64, error) {
	doc, ok := s.docs[id]
	if !ok {
		return nil, nil, fmt.Errorf("not found: %s", id)
	}
	return doc, []float64{1, 2, 3}, nil
}

func (s *insertOnlyVectorStore) Update(_ context.Context, doc *document.Document, _ []float64) error {
	if doc == nil {
		return fmt.Errorf("document required")
	}
	if _, ok := s.docs[doc.ID]; !ok {
		return fmt.Errorf("not found: %s", doc.ID)
	}
	s.docs[doc.ID] = doc
	return nil
}

func (s *insertOnlyVectorStore) Delete(_ context.Context, id string) error {
	delete(s.docs, id)
	return nil
}

func (s *insertOnlyVectorStore) Search(_ context.Context, _ *vectorstore.SearchQuery) (*vectorstore.SearchResult, error) {
	return &vectorstore.SearchResult{}, nil
}

func (s *insertOnlyVectorStore) DeleteByFilter(_ context.Context, opts ...vectorstore.DeleteOption) error {
	config := vectorstore.ApplyDeleteOptions(opts...)
	if len(config.DocumentIDs) == 0 {
		return fmt.Errorf("document ids required")
	}
	ids := slices.Clone(config.DocumentIDs)
	s.deleteIDs = append(s.deleteIDs, ids)
	for _, id := range ids {
		delete(s.docs, id)
	}
	return nil
}

func (s *insertOnlyVectorStore) UpdateByFilter(_ context.Context, _ ...vectorstore.UpdateByFilterOption) (int64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (s *insertOnlyVectorStore) Count(_ context.Context, _ ...vectorstore.CountOption) (int, error) {
	return len(s.docs), nil
}

func (s *insertOnlyVectorStore) GetMetadata(_ context.Context, _ ...vectorstore.GetMetadataOption) (map[string]vectorstore.DocumentMetadata, error) {
	out := make(map[string]vectorstore.DocumentMetadata, len(s.docs))
	for id, doc := range s.docs {
		out[id] = vectorstore.DocumentMetadata{Metadata: doc.Metadata}
	}
	return out, nil
}

func (s *insertOnlyVectorStore) Close() error {
	return nil
}

var _ vectorstore.VectorStore = (*insertOnlyVectorStore)(nil)
