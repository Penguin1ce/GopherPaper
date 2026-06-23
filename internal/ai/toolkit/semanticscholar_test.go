package toolkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// s2Sample 两条结果:第一条带 openAccessPdf,第二条无开放 PDF 但有 ArXiv 编号(走回退)。
const s2Sample = `{"total":2,"data":[
  {"title":"SimCSE","abstract":"contrastive learning","year":2021,"citationCount":1234,
   "venue":"EMNLP","publicationVenue":{"name":"Conference on Empirical Methods in Natural Language Processing"},
   "authors":[{"name":"Tianyu Gao"}],
   "externalIds":{"ArXiv":"2104.08821","DOI":"10.18653/v1/2021.emnlp"},
   "openAccessPdf":{"url":"https://aclanthology.org/2021.emnlp-main.552.pdf"}},
  {"title":"NoPDF Paper","year":2019,"citationCount":7,"venue":"arXiv",
   "authors":[{"name":"A. Author"}],
   "externalIds":{"ArXiv":"1901.00001"},
   "openAccessPdf":{"url":""}}
]}`

// TestSemanticScholarSearch 验证请求参数、key 注入、引用数解析与 pdf_url 回退逻辑。
func TestSemanticScholarSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("query"); got != "sentence embedding" {
			t.Errorf("query = %q", got)
		}
		if got := r.URL.Query().Get("fields"); got != s2Fields {
			t.Errorf("fields = %q", got)
		}
		if got := r.Header.Get("x-api-key"); got != "k-1" {
			t.Errorf("x-api-key = %q", got)
		}
		_, _ = w.Write([]byte(s2Sample))
	}))
	defer srv.Close()
	old := s2BaseURL
	s2BaseURL = srv.URL
	defer func() { s2BaseURL = old }()

	out, err := s2Search(context.Background(), "k-1", s2Input{Query: "sentence embedding"})
	if err != nil {
		t.Fatalf("s2Search: %v", err)
	}
	if len(out.Papers) != 2 {
		t.Fatalf("应解析 2 篇,得到 %d", len(out.Papers))
	}
	a := out.Papers[0]
	if a.CitationCount != 1234 || a.Year != 2021 || a.ArxivID != "2104.08821" {
		t.Errorf("第一条字段不符: %+v", a)
	}
	// venue 应优先取规范化的 publicationVenue.name。
	if a.Venue != "Conference on Empirical Methods in Natural Language Processing" {
		t.Errorf("venue 应取 publicationVenue.name: %q", a.Venue)
	}
	// 第二条无 publicationVenue,应回退自由文本 venue 字段。
	if out.Papers[1].Venue != "arXiv" {
		t.Errorf("第二条 venue 应回退到 venue 字段: %q", out.Papers[1].Venue)
	}
	if a.PDFURL != "https://aclanthology.org/2021.emnlp-main.552.pdf" {
		t.Errorf("第一条 pdf_url 应取 openAccessPdf: %q", a.PDFURL)
	}
	b := out.Papers[1]
	if b.PDFURL != "https://arxiv.org/pdf/1901.00001" {
		t.Errorf("第二条 pdf_url 应回退到 arxiv: %q", b.PDFURL)
	}

	if _, err := s2Search(context.Background(), "", s2Input{}); err == nil {
		t.Fatal("空 query 应报错")
	}
}

// TestSemanticScholarFilters 验证 year/fields_of_study 透传为 S2 query 参数,
// 而 venue 不透传服务端(改走客户端别名模糊匹配)。
func TestSemanticScholarFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("year"); got != "2025-2026" {
			t.Errorf("year = %q", got)
		}
		if _, ok := q["venue"]; ok {
			t.Error("venue 应走客户端过滤,不带 venue 服务端参数")
		}
		if got := q.Get("fieldsOfStudy"); got != "Computer Science" {
			t.Errorf("fieldsOfStudy = %q", got)
		}
		_, _ = w.Write([]byte(s2Sample))
	}))
	defer srv.Close()
	old := s2BaseURL
	s2BaseURL = srv.URL
	defer func() { s2BaseURL = old }()

	_, err := s2Search(context.Background(), "", s2Input{
		Query: "agent", Year: "2025-2026", Venue: "NeurIPS,ICML",
		FieldsOfStudy: "Computer Science",
	})
	if err != nil {
		t.Fatalf("s2Search: %v", err)
	}
}

// TestSemanticScholarVenueClientFilter 验证 venue 走客户端别名模糊匹配:
// 填缩写 EMNLP 应命中 publicationVenue.name 为全称的论文,arXiv 预印本被滤掉。
func TestSemanticScholarVenueClientFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(s2Sample))
	}))
	defer srv.Close()
	old := s2BaseURL
	s2BaseURL = srv.URL
	defer func() { s2BaseURL = old }()

	out, err := s2Search(context.Background(), "", s2Input{Query: "x", Venue: "EMNLP"})
	if err != nil {
		t.Fatalf("s2Search: %v", err)
	}
	// 仅第一条(EMNLP 全称)命中,第二条 venue=arXiv 被滤。
	if len(out.Papers) != 1 {
		t.Fatalf("EMNLP 过滤应只保留 1 篇,得到 %d", len(out.Papers))
	}
	if out.Papers[0].Title != "SimCSE" {
		t.Errorf("应保留 SimCSE,得到 %q", out.Papers[0].Title)
	}
}

// TestSemanticScholarOpenAccessClientFilter 验证 open_access_only 走客户端过滤:
// 不带 openAccessPdf 服务端参数,且只保留兜底后 pdf_url 非空的论文(含 arXiv 镜像)。
func TestSemanticScholarOpenAccessClientFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 不应再透传 openAccessPdf 服务端 flag。
		if _, ok := r.URL.Query()["openAccessPdf"]; ok {
			t.Error("open_access_only 应走客户端过滤,不带 openAccessPdf 服务端参数")
		}
		_, _ = w.Write([]byte(s2Sample))
	}))
	defer srv.Close()
	old := s2BaseURL
	s2BaseURL = srv.URL
	defer func() { s2BaseURL = old }()

	// s2Sample 两条都能下载(第一条有 openAccessPdf,第二条有 arXiv 兜底),都应保留。
	out, err := s2Search(context.Background(), "", s2Input{Query: "x", OpenAccessOnly: true})
	if err != nil {
		t.Fatalf("s2Search: %v", err)
	}
	if len(out.Papers) != 2 {
		t.Fatalf("两条均可下载应都保留,得到 %d", len(out.Papers))
	}
	for _, p := range out.Papers {
		if p.PDFURL == "" {
			t.Errorf("open_access_only 下不应有空 pdf_url: %+v", p)
		}
	}
}

// withFastRetry 把退避节奏临时缩到近零并保留次数,避免测试空等真实秒级退避。
func withFastRetry(t *testing.T) {
	old := s2RetryDelays
	s2RetryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { s2RetryDelays = old })
}

// TestSemanticScholarRateLimited 验证持续 429 重试耗尽后给出可识别的限流错误。
func TestSemanticScholarRateLimited(t *testing.T) {
	withFastRetry(t)
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	old := s2BaseURL
	s2BaseURL = srv.URL
	defer func() { s2BaseURL = old }()

	if _, err := s2Search(context.Background(), "", s2Input{Query: "x"}); err == nil {
		t.Fatal("429 应报错")
	}
	// 首请求 + 3 次重试 = 4 次访问。
	if hits != len(s2RetryDelays)+1 {
		t.Errorf("应重试至耗尽共 %d 次,实际 %d 次", len(s2RetryDelays)+1, hits)
	}
}

// TestSemanticScholarRetryThenSucceed 验证先 429 后放行能自动重试拿到结果。
func TestSemanticScholarRetryThenSucceed(t *testing.T) {
	withFastRetry(t)
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(s2Sample))
	}))
	defer srv.Close()
	old := s2BaseURL
	s2BaseURL = srv.URL
	defer func() { s2BaseURL = old }()

	out, err := s2Search(context.Background(), "", s2Input{Query: "x"})
	if err != nil {
		t.Fatalf("重试后应成功: %v", err)
	}
	if len(out.Papers) != 2 {
		t.Fatalf("应解析 2 篇,得到 %d", len(out.Papers))
	}
	if hits != 2 {
		t.Errorf("应在第 2 次成功,实际访问 %d 次", hits)
	}
}

// TestHostAllowed 验证白名单:精确与子域后缀命中,非白名单与近似域名拒绝。
func TestHostAllowed(t *testing.T) {
	allow := []string{
		"arxiv.org", "www.biorxiv.org", "pdfs.semanticscholar.org",
		"proceedings.mlr.press", "papers.nips.cc", "openaccess.thecvf.com",
		"www.ncbi.nlm.nih.gov", "aclanthology.org",
	}
	for _, h := range allow {
		if !hostAllowed(h) {
			t.Errorf("应放行 %s", h)
		}
	}
	deny := []string{
		"evil.com", "arxiv.org.evil.com", "notarxiv.org", "169.254.169.254", "localhost",
	}
	for _, h := range deny {
		if hostAllowed(h) {
			t.Errorf("应拒绝 %s", h)
		}
	}
}
