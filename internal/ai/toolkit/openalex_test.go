package toolkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// openAlexSample 两条结果:第一条带 best_oa_location 直链 + 倒排摘要;
// 第二条无 best_oa 但 open_access.oa_url 兜底,且 DOI 带 https://doi.org/ 前缀待归一。
const openAlexSample = `{"results":[
  {"id":"https://openalex.org/W2741809807","doi":"https://doi.org/10.7717/peerj.4375",
   "title":"The state of OA","publication_year":2018,"cited_by_count":900,
   "authorships":[{"author":{"display_name":"Heather Piwowar"}}],
   "primary_location":{"landing_page_url":"https://peerj.com/articles/4375","source":{"display_name":"PeerJ"}},
   "best_oa_location":{"pdf_url":"https://arxiv.org/pdf/1802.00001","source":{"display_name":"arXiv"}},
   "open_access":{"oa_url":"https://peerj.com/articles/4375.pdf"},
   "abstract_inverted_index":{"Open":[0],"access":[1],"is":[2],"good":[3]}},
  {"id":"https://openalex.org/W123","doi":"10.1/bare","title":"Bare DOI",
   "publication_year":2020,"cited_by_count":3,
   "authorships":[{"author":{"display_name":"A. Author"}}],
   "open_access":{"oa_url":"https://ncbi.nlm.nih.gov/pmc/articles/PMC1/pdf"}}
]}`

// TestOpenAlexSearch 验证请求参数、key/排序/年份注入、倒排摘要还原、DOI 归一与 pdf_url 回退。
func TestOpenAlexSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("search"); got != "open access" {
			t.Errorf("search = %q", got)
		}
		if got := q.Get("api_key"); got != "k-1" {
			t.Errorf("api_key = %q", got)
		}
		if got := q.Get("sort"); got != "cited_by_count:desc" {
			t.Errorf("sort = %q", got)
		}
		if got := q.Get("filter"); got != "from_publication_date:2015-01-01" {
			t.Errorf("filter = %q", got)
		}
		_, _ = w.Write([]byte(openAlexSample))
	}))
	defer srv.Close()
	old := openAlexBaseURL
	openAlexBaseURL = srv.URL
	defer func() { openAlexBaseURL = old }()

	out, err := openAlexSearch(context.Background(), "k-1",
		openAlexInput{Query: "open access", Sort: "citations", FromYear: 2015})
	if err != nil {
		t.Fatalf("openAlexSearch: %v", err)
	}
	if len(out.Papers) != 2 {
		t.Fatalf("应解析 2 篇,得到 %d", len(out.Papers))
	}
	a := out.Papers[0]
	if a.OpenAlexID != "W2741809807" {
		t.Errorf("openalex_id = %q", a.OpenAlexID)
	}
	if a.DOI != "10.7717/peerj.4375" {
		t.Errorf("doi 未归一: %q", a.DOI)
	}
	if a.Abstract != "Open access is good" {
		t.Errorf("摘要还原错误: %q", a.Abstract)
	}
	if a.PDFURL != "https://arxiv.org/pdf/1802.00001" {
		t.Errorf("pdf_url 应取 best_oa: %q", a.PDFURL)
	}
	if a.Venue != "PeerJ" || a.CitedByCount != 900 {
		t.Errorf("venue/cited 不符: %+v", a)
	}

	b := out.Papers[1]
	if b.DOI != "10.1/bare" {
		t.Errorf("裸 DOI 应原样: %q", b.DOI)
	}
	if b.PDFURL != "https://ncbi.nlm.nih.gov/pmc/articles/PMC1/pdf" {
		t.Errorf("pdf_url 应回退 oa_url: %q", b.PDFURL)
	}
}

// TestOpenAlexAbstractEmpty 倒排索引为空时返回空串,不 panic。
func TestOpenAlexAbstractEmpty(t *testing.T) {
	if got := openAlexAbstract(nil); got != "" {
		t.Errorf("空倒排应返回空串,得到 %q", got)
	}
}
