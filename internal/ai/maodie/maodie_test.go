package maodie

import (
	"context"
	"errors"
	"strings"
	"testing"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/retrieval"
	"GopherPaper/pkg/constant"
)

func TestLocalSearchQueryExpandsShortFollowup(t *testing.T) {
	history := []trpcmodel.Message{
		{Role: trpcmodel.RoleUser, Content: "第一步怎样做?"},
		{Role: trpcmodel.RoleAssistant, Content: "第一步是..."},
	}

	got := localSearchQuery("那第二步呢?", history, "")
	want := "第一步怎样做?\n那第二步呢?"
	if got != want {
		t.Fatalf("localSearchQuery() = %q, want %q", got, want)
	}
}

func TestLocalSearchQueryKeepsLongQuestionStandalone(t *testing.T) {
	history := []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: "上一轮问题"}}
	query := strings.Repeat("这", constant.MaodieFollowupQueryMaxRunes+1)

	got := localSearchQuery(query, history, "")
	if got != query {
		t.Fatalf("localSearchQuery() = %q, want %q", got, query)
	}
}

func TestLocalSearchQueryAppendsSelectionAfterFollowup(t *testing.T) {
	history := []trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: "方法第一步是什么?"}}

	got := localSearchQuery("第二步呢?", history, "selected text")
	want := "方法第一步是什么?\n第二步呢?\nselected text"
	if got != want {
		t.Fatalf("localSearchQuery() = %q, want %q", got, want)
	}
}

func TestLocalSearchQueryUsesSelectionWithoutQuery(t *testing.T) {
	got := localSearchQuery("", nil, " selected text ")
	if got != "selected text" {
		t.Fatalf("localSearchQuery() = %q, want selected text", got)
	}
}

func TestNormalizeReaderContextWhitelistsScope(t *testing.T) {
	got := normalizeReaderContext(core.ReaderContext{Scope: "hack", PageNo: 3, SelectedText: "inject"})
	if got.Scope != "page" || got.SelectedText != "" || got.PageNo != 3 {
		t.Fatalf("normalizeReaderContext() = %+v, want page without selected text", got)
	}

	got = normalizeReaderContext(core.ReaderContext{Scope: "paper", PageNo: 9, SelectedText: "ignored"})
	if got.Scope != "paper" || got.PageNo != 0 || got.SelectedText != "" {
		t.Fatalf("paper scope normalize = %+v, want page_no 0 and no selected text", got)
	}

	got = normalizeReaderContext(core.ReaderContext{Scope: "selection", PageNo: 2})
	if got.Scope != "page" {
		t.Fatalf("empty selection scope should fall back to page, got %+v", got)
	}
}

func TestRetrieveLocalDocsFallsBackToPaperOnPageError(t *testing.T) {
	oldPage, oldPaper := retrievePageForPaper, retrieveForPaper
	defer func() {
		retrievePageForPaper = oldPage
		retrieveForPaper = oldPaper
	}()

	retrievePageForPaper = func(context.Context, string, string, string, int) ([]*retrieval.Doc, error) {
		return nil, errors.New("page query failed")
	}
	retrieveForPaper = func(context.Context, string, string, string) ([]*retrieval.Doc, error) {
		return []*retrieval.Doc{{
			ID:      "paper",
			Content: "全文补充",
			MetaData: map[string]any{
				constant.MilvusFieldPageNo: 7,
			},
		}}, nil
	}

	docs := retrieveLocalDocs(context.Background(), "owner", "paper-1", "query", nil, core.ReaderContext{
		Scope:  "page",
		PageNo: 5,
	})
	if len(docs) != 1 {
		t.Fatalf("docs len=%d, want 1", len(docs))
	}
	if got := retrieval.MetaString(docs[0], constant.MetaKeyFallbackScope); got != "paper" {
		t.Fatalf("fallback_scope=%q, want paper", got)
	}
}

func TestImageDocsOnPageKeepsOnlyCurrentPageImages(t *testing.T) {
	imgDocs := []*retrieval.Doc{
		{ID: "neighbor", MetaData: map[string]any{constant.MilvusFieldPageNo: 2}},
		{ID: "current", MetaData: map[string]any{constant.MilvusFieldPageNo: 3}},
		{ID: "cross", MetaData: map[string]any{constant.MilvusFieldPageNo: 2, constant.MilvusFieldPageEnd: 4}},
	}

	got := imageDocsOnPage(imgDocs, 3)
	if len(got) != 2 || got[0].ID != "current" || got[1].ID != "cross" {
		ids := make([]string, 0, len(got))
		for _, d := range got {
			ids = append(ids, d.ID)
		}
		t.Fatalf("imageDocsOnPage = %v, want [current cross]", ids)
	}
}

func TestModelQueryBindsSelectionToQuestion(t *testing.T) {
	rc := core.ReaderContext{Scope: "selection", PageNo: 3, SelectedText: "RELATED WORK"}

	got := modelQuery("讲讲这个", rc)
	for _, want := range []string{"第 3 页", "RELATED WORK", "问题: 讲讲这个", "「这个/这段/这里」"} {
		if !strings.Contains(got, want) {
			t.Fatalf("modelQuery 缺少 %q:\n%s", want, got)
		}
	}
	if plain := modelQuery("讲讲这个", core.ReaderContext{Scope: "page", PageNo: 3}); plain != "讲讲这个" {
		t.Fatalf("无选段时应原样返回, got %q", plain)
	}
}
