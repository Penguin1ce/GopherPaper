package toolkit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// sciVerseSample 两条命中:第一条 chunk 带 MinerU 图片占位待剔除;第二条字段较全。
const sciVerseSample = `{"biz_code":0,"code":"SUCCESS","message":"ok","hits":[
  {"title":"RAG for Science","chunk":"![](dt=2026/x.jpg)\nRetrieval augmented generation integrates\nexternal evidence.",
   "abstract":"A study on RAG.","publication_published_year":2024,
   "publication_venue_name_unified":"NeurIPS","citation_count":42,
   "primary_topic":"machine learning","page_no":7,"doc_id":"doc-1","score":0.91},
  {"title":"Diffusion Models","chunk":"Diffusion models emerged as powerful tools.",
   "publication_published_year":2023,"publication_venue_name_unified":"ICML",
   "citation_count":83,"primary_topic":"generative models","page_no":3,"doc_id":"doc-2","score":0.8}
]}`

// TestSciVerseSearch 验证请求体、Bearer 注入、top_k 默认与上限、chunk 图片剔除与字段映射。
func TestSciVerseSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer k-1" {
			t.Errorf("Authorization = %q", got)
		}
		var body sciVerseRequest
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Query != "what is rag" {
			t.Errorf("query = %q", body.Query)
		}
		if body.TopK != 5 {
			t.Errorf("top_k 默认应为 5, got %d", body.TopK)
		}
		if body.Retrieval != "hybrid" {
			t.Errorf("retrieval = %q", body.Retrieval)
		}
		_, _ = w.Write([]byte(sciVerseSample))
	}))
	defer srv.Close()
	old := sciVerseBaseURL
	sciVerseBaseURL = srv.URL
	defer func() { sciVerseBaseURL = old }()

	out, err := sciVerseSearch(context.Background(), "k-1", sciVerseInput{Query: "what is rag"})
	if err != nil {
		t.Fatalf("sciVerseSearch: %v", err)
	}
	if len(out.Hits) != 2 {
		t.Fatalf("应解析 2 条, 得到 %d", len(out.Hits))
	}
	a := out.Hits[0]
	if a.Title != "RAG for Science" {
		t.Errorf("title = %q", a.Title)
	}
	if a.Snippet != "Retrieval augmented generation integrates external evidence." {
		t.Errorf("snippet 未剔除图片或归一空白: %q", a.Snippet)
	}
	if a.Venue != "NeurIPS" || a.Year != 2024 || a.CitedByCount != 42 || a.PageNo != 7 {
		t.Errorf("字段映射不符: %+v", a)
	}
	if a.DocID != "doc-1" || a.Score != 0.91 {
		t.Errorf("doc_id/score 不符: %+v", a)
	}
}

// TestSciVerseSearchCapAndBizError 验证 max_results 上限收敛与业务错误码透出。
func TestSciVerseSearchCapAndBizError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body sciVerseRequest
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		if body.TopK != 10 {
			t.Errorf("top_k 应收敛到上限 10, got %d", body.TopK)
		}
		_, _ = w.Write([]byte(`{"biz_code":1001,"code":"QUOTA_EXCEEDED","message":"额度不足","hits":[]}`))
	}))
	defer srv.Close()
	old := sciVerseBaseURL
	sciVerseBaseURL = srv.URL
	defer func() { sciVerseBaseURL = old }()

	_, err := sciVerseSearch(context.Background(), "k-1", sciVerseInput{Query: "q", MaxResults: 50})
	if err == nil {
		t.Fatal("biz_code 非 0 应返回错误")
	}
}

// TestSciVerseScoreFloor 低于相关度下限的命中被丢弃,只有全是低分时才返回空。
func TestSciVerseScoreFloor(t *testing.T) {
	// 一条 0.95 相关、一条 0.42 噪声(模拟 top_k 凑数)。
	const sample = `{"biz_code":0,"code":"SUCCESS","hits":[
	  {"title":"Relevant","chunk":"on topic","score":0.95},
	  {"title":"Noise","chunk":"off topic","score":0.42}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sample))
	}))
	defer srv.Close()
	old := sciVerseBaseURL
	sciVerseBaseURL = srv.URL
	defer func() { sciVerseBaseURL = old }()

	out, err := sciVerseSearch(context.Background(), "k-1", sciVerseInput{Query: "topic"})
	if err != nil {
		t.Fatalf("sciVerseSearch: %v", err)
	}
	if len(out.Hits) != 1 || out.Hits[0].Title != "Relevant" {
		t.Fatalf("应只保留高分命中, 得到 %+v", out.Hits)
	}
}

// TestReadSciVerseContent 验证续读:doc_id/offset/limit 透传、limit 上限收敛、text 清洗与 more 透出。
func TestReadSciVerseContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("doc_id") != "doc-1" {
			t.Errorf("doc_id = %q", q.Get("doc_id"))
		}
		if q.Get("offset") != "120" {
			t.Errorf("offset = %q", q.Get("offset"))
		}
		if q.Get("limit") != "6000" {
			t.Errorf("limit 应收敛到上限 6000, got %q", q.Get("limit"))
		}
		if got := r.Header.Get("Authorization"); got != "Bearer k-1" {
			t.Errorf("Authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"text":"![](x.jpg)\nexpanded   context here","next_offset":1620,"more":true,"text_length":99999}`))
	}))
	defer srv.Close()
	old := sciVerseContentURL
	sciVerseContentURL = srv.URL
	defer func() { sciVerseContentURL = old }()

	out, err := readSciVerseContent(context.Background(), "k-1",
		sciVerseContentInput{DocID: "doc-1", Offset: 120, Limit: 999999})
	if err != nil {
		t.Fatalf("readSciVerseContent: %v", err)
	}
	if out.Text != "expanded context here" {
		t.Errorf("text 未清洗/归一: %q", out.Text)
	}
	if out.NextOffset != 1620 || !out.More || out.TextLength != 99999 {
		t.Errorf("分页字段不符: %+v", out)
	}
}

// TestReadSciVerseContentEmptyDocID 空 doc_id 直接报错,不发请求。
func TestReadSciVerseContentEmptyDocID(t *testing.T) {
	if _, err := readSciVerseContent(context.Background(), "k-1", sciVerseContentInput{DocID: " "}); err == nil {
		t.Fatal("空 doc_id 应报错")
	}
}

// TestSciVerseSearchEmptyQuery 空 query 直接报错,不发请求。
func TestSciVerseSearchEmptyQuery(t *testing.T) {
	if _, err := sciVerseSearch(context.Background(), "k-1", sciVerseInput{Query: "  "}); err == nil {
		t.Fatal("空 query 应报错")
	}
}
