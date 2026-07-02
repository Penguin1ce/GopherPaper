package retrieval

import (
	"context"
	"slices"
	"strings"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/knowledge/document"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"

	"GopherPaper/pkg/constant"
)

func TestRetrievePageForPaperQueriesRangeOnceAndOrdersCurrentPageFirst(t *testing.T) {
	restore := installPageRetrievalStubs()
	defer restore()

	var queriedRanges [][2]int
	fallbackCalled := false
	queryPageKnowledge = func(_ context.Context, _ string, _ string, pageStart, pageEnd, _ int) (*vectorstore.SearchResult, error) {
		queriedRanges = append(queriedRanges, [2]int{pageStart, pageEnd})
		return searchResult(
			scoredDoc("prev", "上一页补充", pageMeta(4, 40), 0),
			scoredDoc("target-2", "目标页内容 2", pageMeta(5, 52), 0),
			scoredDoc("cross", "跨页块", pageMetaRange(4, 6, 51), 0),
			scoredDoc("target-1", "目标页内容 1", pageMeta(5, 50), 0),
		), nil
	}
	searchKnowledge = func(context.Context, string, string, string, int) (*vectorstore.SearchResult, error) {
		fallbackCalled = true
		return searchResult(scoredDoc("paper", "全文补充", pageMeta(9, 90), 0.7)), nil
	}

	docs, err := RetrievePageForPaper(context.Background(), "query", "owner", "paper-1", 5)
	if err != nil {
		t.Fatalf("RetrievePageForPaper: %v", err)
	}
	if fallbackCalled {
		t.Fatal("页邻域有证据时不应触发全文补充")
	}
	if len(queriedRanges) != 1 || queriedRanges[0] != [2]int{4, 6} {
		t.Fatalf("查询区间=%v, want 一次 [4 6]", queriedRanges)
	}
	// 当前页块(含跨页块)稳定排前并按 chunk_index 排序,邻页块靠后。
	if got := docIDs(docs); !slices.Equal(got, []string{"target-1", "cross", "target-2", "prev"}) {
		t.Fatalf("docs=%v, want [target-1 cross target-2 prev]", got)
	}
}

func TestRetrievePageForPaperClampsRangeAtFirstPage(t *testing.T) {
	restore := installPageRetrievalStubs()
	defer restore()

	var searchedRange [2]int
	queryPageKnowledge = func(_ context.Context, _ string, _ string, pageStart, pageEnd, _ int) (*vectorstore.SearchResult, error) {
		searchedRange = [2]int{pageStart, pageEnd}
		return searchResult(scoredDoc("target", "首页内容", pageMeta(1, 1), 0)), nil
	}
	searchKnowledge = func(context.Context, string, string, string, int) (*vectorstore.SearchResult, error) {
		t.Fatal("不应触发全文补充")
		return nil, nil
	}

	if _, err := RetrievePageForPaper(context.Background(), "query", "owner", "paper-1", 1); err != nil {
		t.Fatalf("RetrievePageForPaper: %v", err)
	}
	if searchedRange != [2]int{1, 2} {
		t.Fatalf("检索区间=%v, want [1 2]", searchedRange)
	}
}

func TestRetrievePageForPaperMarksPaperFallback(t *testing.T) {
	restore := installPageRetrievalStubs()
	defer restore()

	queryPageKnowledge = func(context.Context, string, string, int, int, int) (*vectorstore.SearchResult, error) {
		return searchResult(), nil
	}
	searchKnowledge = func(context.Context, string, string, string, int) (*vectorstore.SearchResult, error) {
		return searchResult(scoredDoc("paper", "全文补充", pageMeta(9, 90), 0.7)), nil
	}

	docs, err := RetrievePageForPaper(context.Background(), "query", "owner", "paper-1", 5)
	if err != nil {
		t.Fatalf("RetrievePageForPaper: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("docs len=%d, want 1", len(docs))
	}
	if got := MetaString(docs[0], constant.MetaKeyFallbackScope); got != "paper" {
		t.Fatalf("fallback_scope=%q, want paper", got)
	}
	refs := References(docs)
	if len(refs) != 1 || refs[0].Fallback != "paper" {
		t.Fatalf("refs fallback 未透出: %+v", refs)
	}
}

func TestRetrievePageForPaperKeepsAllCurrentPageAndBudgetsNeighbors(t *testing.T) {
	restore := installPageRetrievalStubs()
	defer restore()

	bigNeighbor := strings.Repeat("邻", constant.MaodiePageContextMaxRunes)
	queryPageKnowledge = func(context.Context, string, string, int, int, int) (*vectorstore.SearchResult, error) {
		return searchResult(
			scoredDoc("target-a", "当前页 A", pageMeta(5, 50), 0),
			scoredDoc("target-b", strings.Repeat("本", constant.MaodiePageContextMaxRunes), pageMeta(5, 51), 0),
			scoredDoc("neighbor-big", bigNeighbor, pageMeta(6, 60), 0),
		), nil
	}
	searchKnowledge = func(context.Context, string, string, string, int) (*vectorstore.SearchResult, error) {
		t.Fatal("页块非空时不应触发全文补充")
		return nil, nil
	}

	docs, err := RetrievePageForPaper(context.Background(), "query", "owner", "paper-1", 5)
	if err != nil {
		t.Fatalf("RetrievePageForPaper: %v", err)
	}
	if got := docIDs(docs); !slices.Equal(got, []string{"target-a", "target-b"}) {
		t.Fatalf("docs=%v, want 只保留当前页完整块并跳过超预算邻页", got)
	}
}

func installPageRetrievalStubs() func() {
	oldSearch, oldPage, oldReranker := searchKnowledge, queryPageKnowledge, reranker
	reranker = nil
	return func() {
		searchKnowledge = oldSearch
		queryPageKnowledge = oldPage
		reranker = oldReranker
	}
}

func searchResult(results ...*vectorstore.ScoredDocument) *vectorstore.SearchResult {
	return &vectorstore.SearchResult{Results: results}
}

func scoredDoc(id, content string, meta map[string]any, score float64) *vectorstore.ScoredDocument {
	return &vectorstore.ScoredDocument{
		Document: &document.Document{ID: id, Content: content, Metadata: meta},
		Score:    score,
	}
}

func pageMeta(page, chunkIndex int) map[string]any {
	return map[string]any{
		constant.MilvusFieldKnowledgeScope: constant.KnowledgeScopePrivate,
		constant.MilvusFieldDocID:          "paper-1",
		constant.MilvusFieldPageNo:         page,
		constant.MilvusFieldChunkIndex:     chunkIndex,
	}
}

func pageMetaRange(page, pageEnd, chunkIndex int) map[string]any {
	meta := pageMeta(page, chunkIndex)
	meta[constant.MilvusFieldPageEnd] = pageEnd
	return meta
}

func docIDs(docs []*Doc) []string {
	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}
	return ids
}
