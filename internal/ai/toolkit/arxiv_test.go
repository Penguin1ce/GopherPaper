package toolkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// arxivSample 是一条精简的 arXiv Atom 响应,覆盖 id/标题/作者/摘要/时间。
const arxivSample = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2503.03480v1</id>
    <published>2025-03-05T18:59:59Z</published>
    <title>Attention Is
      All You Need</title>
    <summary>  We propose a new
      architecture.  </summary>
    <author><name>Ashish Vaswani</name></author>
    <author><name>Noam Shazeer</name></author>
  </entry>
</feed>`

// TestArxivSearch 验证查询参数拼装(无前缀补 all:)与 Atom 解析、pdf_url 拼接。
func TestArxivSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("search_query"); got != "all:transformer" {
			t.Errorf("search_query = %q, 期望 all:transformer", got)
		}
		if got := r.URL.Query().Get("max_results"); got != "3" {
			t.Errorf("max_results = %q", got)
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(arxivSample))
	}))
	defer srv.Close()
	old := arxivBaseURL
	arxivBaseURL = srv.URL
	defer func() { arxivBaseURL = old }()

	out, err := arxivSearch(context.Background(), arxivInput{Query: "transformer", MaxResults: 3})
	if err != nil {
		t.Fatalf("arxivSearch: %v", err)
	}
	if len(out.Papers) != 1 {
		t.Fatalf("应解析出 1 篇,得到 %d", len(out.Papers))
	}
	p := out.Papers[0]
	if p.ArxivID != "2503.03480v1" {
		t.Errorf("arxiv_id = %q", p.ArxivID)
	}
	if p.Title != "Attention Is All You Need" {
		t.Errorf("title 未折叠空白: %q", p.Title)
	}
	if p.Abstract != "We propose a new architecture." {
		t.Errorf("abstract 未折叠空白: %q", p.Abstract)
	}
	if p.Published != "2025-03-05" {
		t.Errorf("published = %q", p.Published)
	}
	if p.PDFURL != "https://arxiv.org/pdf/2503.03480v1" {
		t.Errorf("pdf_url = %q", p.PDFURL)
	}
	if len(p.Authors) != 2 || p.Authors[0] != "Ashish Vaswani" {
		t.Errorf("authors = %v", p.Authors)
	}

	if _, err := arxivSearch(context.Background(), arxivInput{}); err == nil {
		t.Fatal("空 query 应报错")
	}
}

// TestArxivFieldQueryPassthrough 验证带字段前缀的查询原样透传,不再套 all:。
func TestArxivFieldQueryPassthrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("search_query"); got != "ti:bert" {
			t.Errorf("search_query = %q, 期望原样 ti:bert", got)
		}
		_, _ = w.Write([]byte(arxivSample))
	}))
	defer srv.Close()
	old := arxivBaseURL
	arxivBaseURL = srv.URL
	defer func() { arxivBaseURL = old }()

	if _, err := arxivSearch(context.Background(), arxivInput{Query: "ti:bert"}); err != nil {
		t.Fatalf("arxivSearch: %v", err)
	}
}

// TestArxivFilters 验证分类、提交时间区间与 recency 排序拼进检索式与排序参数。
func TestArxivFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		wantSQ := "all:agent AND (cat:cs.AI OR cat:cs.CV) AND submittedDate:[202501010000 TO 202612312359]"
		if got := q.Get("search_query"); got != wantSQ {
			t.Errorf("search_query = %q\n期望 %q", got, wantSQ)
		}
		if got := q.Get("sortBy"); got != "submittedDate" {
			t.Errorf("sortBy = %q, 期望 submittedDate", got)
		}
		if got := q.Get("sortOrder"); got != "descending" {
			t.Errorf("sortOrder = %q, 期望 descending", got)
		}
		_, _ = w.Write([]byte(arxivSample))
	}))
	defer srv.Close()
	old := arxivBaseURL
	arxivBaseURL = srv.URL
	defer func() { arxivBaseURL = old }()

	_, err := arxivSearch(context.Background(), arxivInput{
		Query: "agent", Categories: "cs.AI, cs.CV", FromYear: 2025, ToYear: 2026, Sort: "recency",
	})
	if err != nil {
		t.Fatalf("arxivSearch: %v", err)
	}
}

// TestArxivCategoryClause 验证分类子句:单个不套括号,多个用 OR 套括号,空则空串。
func TestArxivCategoryClause(t *testing.T) {
	cases := map[string]string{
		"":              "",
		"  ":            "",
		"cs.AI":         "cat:cs.AI",
		"cs.AI, cs.CL ": "(cat:cs.AI OR cat:cs.CL)",
	}
	for in, want := range cases {
		if got := arxivCategoryClause(in); got != want {
			t.Errorf("arxivCategoryClause(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestArxivDateClause 验证时间区间:双端、单端取宽边界、空。
func TestArxivDateClause(t *testing.T) {
	cases := []struct {
		from, to int
		want     string
	}{
		{0, 0, ""},
		{2025, 2026, "submittedDate:[202501010000 TO 202612312359]"},
		{2024, 0, "submittedDate:[202401010000 TO 210012312359]"},
		{0, 2020, "submittedDate:[190001010000 TO 202012312359]"},
	}
	for _, c := range cases {
		if got := arxivDateClause(c.from, c.to); got != c.want {
			t.Errorf("arxivDateClause(%d,%d) = %q, want %q", c.from, c.to, got, c.want)
		}
	}
}
