package toolkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNormalizeS2PaperID 验证论文标识规整:带前缀/裸 paperId 原样,裸 arXiv 补 ARXIV:,裸 DOI 补 DOI:。
func TestNormalizeS2PaperID(t *testing.T) {
	cases := map[string]string{
		"  2106.15928  ":                           "ARXIV:2106.15928", // 裸 arXiv 编号
		"2104.08821v2":                             "ARXIV:2104.08821", // 带版本号去掉 v2,S2 的 ARXIV: 不认版本
		"10.18653/v1/2021.emnlp":                   "DOI:10.18653/v1/2021.emnlp",
		"ARXIV:2106.15928":                         "ARXIV:2106.15928", // 已带前缀
		"CorpusId:215416146":                       "CorpusId:215416146",
		"649def34f8be52c8b66281af98ae884c33cd1410": "649def34f8be52c8b66281af98ae884c33cd1410", // 裸 S2 paperId
		"": "",
	}
	for in, want := range cases {
		if got := normalizeS2PaperID(in); got != want {
			t.Errorf("normalizeS2PaperID(%q) = %q, want %q", in, got, want)
		}
	}
}

// s2RelatedRec 是一条可复用的论文记录:带 paperId、开放 PDF 与 EMNLP 场所。
const s2RelatedRec = `{"paperId":"abc123","title":"SimCSE","year":2021,"citationCount":1234,
  "venue":"EMNLP","publicationVenue":{"name":"Conference on Empirical Methods in Natural Language Processing"},
  "authors":[{"name":"Tianyu Gao"}],"externalIds":{"ArXiv":"2104.08821","DOI":"10.18653/v1/2021.emnlp"},
  "openAccessPdf":{"url":"https://aclanthology.org/2021.emnlp-main.552.pdf"}}`

// s2RelatedNoPDF 是一条无开放 PDF 也无 arXiv 的记录,用于验证 open_access_only 过滤。
const s2RelatedNoPDF = `{"paperId":"nopdf","title":"Closed Paper","year":2019,"citationCount":3,
  "authors":[{"name":"A. Author"}],"externalIds":{},"openAccessPdf":{"url":""}}`

// TestS2Citations 验证 citations 端点:路径含规整后的 ID、解析嵌套 citingPaper、透出 paper_id 与 pdf_url。
func TestS2Citations(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if got := r.URL.Query().Get("fields"); got != s2Fields {
			t.Errorf("fields = %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"citingPaper":` + s2RelatedRec + `}]}`))
	}))
	defer srv.Close()
	old := s2PaperBaseURL
	s2PaperBaseURL = srv.URL
	defer func() { s2PaperBaseURL = old }()

	out, err := s2Citations(context.Background(), "", s2RelatedInput{PaperID: "2104.08821"})
	if err != nil {
		t.Fatalf("s2Citations: %v", err)
	}
	// 裸 arXiv 编号应补前缀进 URL 路径(冒号在 path segment 不转义)。
	if !strings.HasSuffix(gotPath, "/ARXIV:2104.08821/citations") {
		t.Errorf("path = %q, 应含规整后的 ID", gotPath)
	}
	if len(out.Papers) != 1 {
		t.Fatalf("应解析 1 篇,得到 %d", len(out.Papers))
	}
	p := out.Papers[0]
	if p.PaperID != "abc123" {
		t.Errorf("paper_id = %q", p.PaperID)
	}
	if p.PDFURL != "https://aclanthology.org/2021.emnlp-main.552.pdf" {
		t.Errorf("pdf_url = %q", p.PDFURL)
	}
	if p.CitationCount != 1234 {
		t.Errorf("citation_count = %d", p.CitationCount)
	}
}

// TestS2References 验证 references 端点:DOI 标识的斜杠在 path 里被转义,解析嵌套 citedPaper。
func TestS2References(t *testing.T) {
	var gotRawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"data":[{"citedPaper":` + s2RelatedRec + `}]}`))
	}))
	defer srv.Close()
	old := s2PaperBaseURL
	s2PaperBaseURL = srv.URL
	defer func() { s2PaperBaseURL = old }()

	out, err := s2References(context.Background(), "", s2RelatedInput{PaperID: "10.18653/v1/2021.emnlp"})
	if err != nil {
		t.Fatalf("s2References: %v", err)
	}
	// DOI 的斜杠必须转义成 %2F,否则会被当成额外的 path segment。
	if !strings.Contains(gotRawPath, "DOI:10.18653%2Fv1%2F2021.emnlp/references") {
		t.Errorf("escaped path = %q, DOI 斜杠应转义", gotRawPath)
	}
	if len(out.Papers) != 1 || out.Papers[0].Title != "SimCSE" {
		t.Fatalf("应解析 1 篇 SimCSE,得到 %+v", out.Papers)
	}
}

// TestS2Recommend 验证推荐端点:走 recommend 基址,解析 recommendedPapers,open_access_only 滤掉无 PDF 项。
func TestS2Recommend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"recommendedPapers":[` + s2RelatedRec + `,` + s2RelatedNoPDF + `]}`))
	}))
	defer srv.Close()
	old := s2RecommendBaseURL
	s2RecommendBaseURL = srv.URL
	defer func() { s2RecommendBaseURL = old }()

	// 不设 open_access_only:两条都返回。
	out, err := s2Recommend(context.Background(), "", s2RelatedInput{PaperID: "abc123"})
	if err != nil {
		t.Fatalf("s2Recommend: %v", err)
	}
	if len(out.Papers) != 2 {
		t.Fatalf("应解析 2 篇,得到 %d", len(out.Papers))
	}

	// 设 open_access_only:无 PDF 的 Closed Paper 被滤掉,只剩 1 篇。
	out, err = s2Recommend(context.Background(), "", s2RelatedInput{PaperID: "abc123", OpenAccessOnly: true})
	if err != nil {
		t.Fatalf("s2Recommend(open): %v", err)
	}
	if len(out.Papers) != 1 || out.Papers[0].PaperID != "abc123" {
		t.Fatalf("open_access_only 应只留 1 篇可下载,得到 %+v", out.Papers)
	}
}

// TestS2RelatedMaxResults 验证条数截断:返回多于 max_results 时截到 n。
func TestS2RelatedMaxResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[
		  {"citingPaper":` + s2RelatedRec + `},
		  {"citingPaper":` + s2RelatedRec + `},
		  {"citingPaper":` + s2RelatedRec + `}]}`))
	}))
	defer srv.Close()
	old := s2PaperBaseURL
	s2PaperBaseURL = srv.URL
	defer func() { s2PaperBaseURL = old }()

	out, err := s2Citations(context.Background(), "", s2RelatedInput{PaperID: "abc123", MaxResults: 2})
	if err != nil {
		t.Fatalf("s2Citations: %v", err)
	}
	if len(out.Papers) != 2 {
		t.Fatalf("应截断到 2 篇,得到 %d", len(out.Papers))
	}
}

// TestS2RelatedEmptyID 验证空 paper_id 直接报错,不发请求。
func TestS2RelatedEmptyID(t *testing.T) {
	if _, err := s2Recommend(context.Background(), "", s2RelatedInput{PaperID: "   "}); err == nil {
		t.Fatal("空 paper_id 应报错")
	}
}

// TestS2ProxyEndpointsAndBearer 验证配中转基址后三端点改写到代理之下,且鉴权切到 Bearer。
func TestS2ProxyEndpointsAndBearer(t *testing.T) {
	// 保存并在结束后还原所有被 setS2Endpoints 改动的包级状态。
	oldSearch, oldPaper, oldRec := s2BaseURL, s2PaperBaseURL, s2RecommendBaseURL
	oldBearer, oldInterval := s2AuthBearer, s2MinInterval
	t.Cleanup(func() {
		s2BaseURL, s2PaperBaseURL, s2RecommendBaseURL = oldSearch, oldPaper, oldRec
		s2AuthBearer, s2MinInterval = oldBearer, oldInterval
	})

	var gotAuth, gotXAPIKey, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotXAPIKey = r.Header.Get("x-api-key")
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"data":[{"citedPaper":` + s2RelatedRec + `}]}`))
	}))
	defer srv.Close()

	// 带末尾斜杠,验证被裁掉。
	setS2Endpoints(srv.URL + "/")
	s2MinInterval = 0

	if _, err := s2References(context.Background(), "sk-token", s2RelatedInput{PaperID: "abc123"}); err != nil {
		t.Fatalf("s2References: %v", err)
	}
	if gotAuth != "Bearer sk-token" {
		t.Errorf("Authorization = %q, 应为 Bearer sk-token", gotAuth)
	}
	if gotXAPIKey != "" {
		t.Errorf("不该再发 x-api-key 头,得到 %q", gotXAPIKey)
	}
	if gotPath != "/graph/v1/paper/abc123/references" {
		t.Errorf("path = %q, 应改写到代理 /graph/v1 路径下", gotPath)
	}
}

// TestS2RelatedRejectsLocalUUID 验证传本站论文 UUID 时提前报错并引导,不发请求。
func TestS2RelatedRejectsLocalUUID(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	old := s2PaperBaseURL
	s2PaperBaseURL = srv.URL
	defer func() { s2PaperBaseURL = old }()

	_, err := s2References(context.Background(), "", s2RelatedInput{PaperID: "b328f096-278f-4bbb-b1e8-428c9c796c5e"})
	if err == nil || !strings.Contains(err.Error(), "本站论文 ID") {
		t.Fatalf("本站 UUID 应提前报错并点破,得到 %v", err)
	}
	if hit {
		t.Error("不该向 S2 发请求")
	}
}

// TestS2RelatedNotFound 验证 404 译成可读的"未找到论文"提示。
func TestS2RelatedNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Paper not found"}`))
	}))
	defer srv.Close()
	old := s2PaperBaseURL
	s2PaperBaseURL = srv.URL
	defer func() { s2PaperBaseURL = old }()

	_, err := s2Citations(context.Background(), "", s2RelatedInput{PaperID: "abc123"})
	if err == nil || !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("404 应给出未找到提示,得到 %v", err)
	}
}
