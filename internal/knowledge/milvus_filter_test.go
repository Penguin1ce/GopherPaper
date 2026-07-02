package knowledge

import (
	"context"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/knowledge/searchfilter"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"

	"GopherPaper/pkg/constant"
)

func TestPageRangeConditionMatchesInRangeAndCrossPageChunks(t *testing.T) {
	cond := pageRangeCondition(4, 6)
	if cond.Operator != searchfilter.OperatorOr {
		t.Fatalf("operator=%q, want or", cond.Operator)
	}
	children, ok := cond.Value.([]*searchfilter.UniversalFilterCondition)
	if !ok || len(children) != 2 {
		t.Fatalf("or children=%T/%v, want 2 conditions", cond.Value, cond.Value)
	}

	pageField := metadataPrefix + constant.MilvusFieldPageNo
	endField := metadataPrefix + constant.MilvusFieldPageEnd

	inRange, ok := children[0].Value.([]*searchfilter.UniversalFilterCondition)
	if children[0].Operator != searchfilter.OperatorAnd || !ok || len(inRange) != 2 {
		t.Fatalf("in-range branch=%+v, want and(page_no>=4,page_no<=6)", children[0])
	}
	assertCondition(t, inRange[0], pageField, searchfilter.OperatorGreaterThanOrEqual, 4)
	assertCondition(t, inRange[1], pageField, searchfilter.OperatorLessThanOrEqual, 6)

	cross, ok := children[1].Value.([]*searchfilter.UniversalFilterCondition)
	if children[1].Operator != searchfilter.OperatorAnd || !ok || len(cross) != 2 {
		t.Fatalf("cross-page branch=%+v, want and(page_no<=6,page_end>=4)", children[1])
	}
	assertCondition(t, cross[0], pageField, searchfilter.OperatorLessThanOrEqual, 6)
	assertCondition(t, cross[1], endField, searchfilter.OperatorGreaterThanOrEqual, 4)
}

func TestQueryPageRangeUsesFilterOnlySearch(t *testing.T) {
	restore := installTestKnowledgeStore()
	defer restore()

	if _, err := QueryPageRange(context.Background(), "owner-1", "paper-1", 4, 6, 64); err != nil {
		t.Fatalf("QueryPageRange: %v", err)
	}
	store := trpcStore.(*insertOnlyVectorStore)
	if len(store.searchQueries) != 1 {
		t.Fatalf("Search 调用次数=%d, want 1", len(store.searchQueries))
	}
	q := store.searchQueries[0]
	if q.SearchMode != vectorstore.SearchModeFilter {
		t.Fatalf("SearchMode=%v, want filter", q.SearchMode)
	}
	if q.Query != "" || len(q.Vector) != 0 {
		t.Fatalf("标量 Query 不应带语义 query/vector, got query=%q vector=%v", q.Query, q.Vector)
	}
	if q.Limit != 64 {
		t.Fatalf("Limit=%d, want 64", q.Limit)
	}
	if q.Filter == nil || q.Filter.FilterCondition == nil {
		t.Fatal("缺少 FilterCondition")
	}
	if q.Filter.FilterCondition.Operator != searchfilter.OperatorAnd {
		t.Fatalf("顶层 filter=%+v, want and(scope, page-range)", q.Filter.FilterCondition)
	}
}

func assertCondition(t *testing.T, cond *searchfilter.UniversalFilterCondition, field, op string, value any) {
	t.Helper()
	if cond.Field != field || cond.Operator != op || cond.Value != value {
		t.Fatalf("condition=%+v, want field=%s op=%s value=%v", cond, field, op, value)
	}
}
