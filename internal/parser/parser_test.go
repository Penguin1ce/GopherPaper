package parser

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"GopherPaper/internal/config"
)

// buildZip 造一个含 content_list.json 的产物 zip。
func buildZip(t *testing.T, blocks []map[string]any) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("paper_content_list.json")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(blocks)
	if _, err := w.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestParse_EndToEnd 用 mock 服务模拟 MinerU 异步三段式，验证映射。
func TestParse_EndToEnd(t *testing.T) {
	zipBytes := buildZip(t, []map[string]any{
		{"type": "title", "text": "Introduction", "text_level": 1, "page_idx": 0},
		{"type": "text", "text": "This paper studies X.", "page_idx": 0},
		{"type": "image", "img_caption": []string{"Figure 1: arch"}, "page_idx": 1},
		{"type": "title", "text": "References", "text_level": 1, "page_idx": 2},
		{"type": "text", "text": "[1] Some cited work.", "page_idx": 2},
	})

	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/file-urls/batch", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"batch_id":  "b1",
				"file_urls": []string{srv.URL + "/upload"},
			},
		})
	})
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/extract-results/batch/b1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"extract_result": []map[string]any{
					{"file_name": "paper.pdf", "state": "done", "full_zip_url": srv.URL + "/zip"},
				},
			},
		})
	})
	mux.HandleFunc("/zip", func(w http.ResponseWriter, r *http.Request) { w.Write(zipBytes) })
	srv = httptest.NewServer(mux)
	defer srv.Close()

	Init(config.ParserConfig{BaseURL: srv.URL, Token: "t", Timeout: 10, PollInterval: 1, PollTimeout: 10})

	pdf := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.4 fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := Parse(context.Background(), pdf)
	if err != nil {
		t.Fatalf("Parse 失败 %v", err)
	}
	if len(doc.Sections) != 2 {
		t.Fatalf("章节数应为 2，实际 %d", len(doc.Sections))
	}
	if len(doc.Paragraphs) != 1 || doc.Paragraphs[0].PageNo != 1 {
		t.Fatalf("正文段落映射错误 %+v", doc.Paragraphs)
	}
	if doc.Paragraphs[0].SectionPath != "Introduction" {
		t.Fatalf("章节路径错误 %q", doc.Paragraphs[0].SectionPath)
	}
	if len(doc.Figures) != 1 || doc.Figures[0].Caption != "Figure 1: arch" {
		t.Fatalf("图表说明映射错误 %+v", doc.Figures)
	}
	if len(doc.References) != 1 {
		t.Fatalf("参考文献应为 1 条，实际 %d", len(doc.References))
	}
	if doc.PageCount != 3 {
		t.Fatalf("页数应为 3，实际 %d", doc.PageCount)
	}
}
