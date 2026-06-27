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

func TestMapBlocks_CurrentMinerUContentListFormat(t *testing.T) {
	doc := mapBlocks([]contentBlock{
		{Type: "text", Text: "Introduction", TextLevel: 1, PageIdx: 0},
		{Type: "text", Text: "This paper studies X.", PageIdx: 0},
		{Type: "image", ImgPath: "images/fig1.png", ImageCaption: []string{"Figure 1: arch"}, ImageFootnote: []string{"image note"}, PageIdx: 1},
		{Type: "table", TableCaption: []string{"Table 1: results"}, TableFootnote: []string{"table note"}, PageIdx: 1},
		{Type: "text", Text: "References", TextLevel: 1, PageIdx: 2},
		{Type: "text", Text: "[1] Some cited work.", PageIdx: 2},
	}, map[string][]byte{"fig1.png": []byte("png")})

	if len(doc.Sections) != 2 {
		t.Fatalf("章节数应为 2，实际 %d", len(doc.Sections))
	}
	if doc.Sections[0].Title != "Introduction" || doc.Sections[0].Level != 1 {
		t.Fatalf("标题映射错误 %+v", doc.Sections[0])
	}
	if len(doc.Paragraphs) != 1 || doc.Paragraphs[0].SectionPath != "Introduction" {
		t.Fatalf("正文段落映射错误 %+v", doc.Paragraphs)
	}
	if len(doc.Figures) != 2 {
		t.Fatalf("图表数应为 2，实际 %d", len(doc.Figures))
	}
	if doc.Figures[0].Caption != "Figure 1: arch image note" || string(doc.Figures[0].ImgData) != "png" {
		t.Fatalf("图片说明或字节映射错误 %+v", doc.Figures[0])
	}
	if doc.Figures[1].Caption != "Table 1: results table note" {
		t.Fatalf("表格说明映射错误 %+v", doc.Figures[1])
	}
	if len(doc.References) != 1 {
		t.Fatalf("参考文献应为 1 条，实际 %d", len(doc.References))
	}
}

func TestReadArtifacts_ModelJSONReferences(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	content, err := zw.Create("paper_content_list.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := content.Write([]byte(`[{"type":"text","text":"body","page_idx":0}]`)); err != nil {
		t.Fatal(err)
	}
	model, err := zw.Create("paper_model.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := model.Write([]byte(`{"pdf_info":[{"para_blocks":[{"blocks":[{"type":"ref_text","lines":[{"spans":[{"type":"text","content":"[1] W. Dai, b-money, 1998."}]}]}]}]}]}`)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	blocks, images, refs, err := readArtifacts(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	doc := mapBlocks(blocks, images)
	doc.References = mergeReferences(doc.References, refs)

	if len(doc.References) != 1 || doc.References[0] != "[1] W. Dai, b-money, 1998." {
		t.Fatalf("ref_text 引用映射错误 %+v", doc.References)
	}
}

func TestReadArtifacts_CaptionStringOrArray(t *testing.T) {
	zipBytes := buildZip(t, []map[string]any{
		{
			"type":           "image",
			"img_path":       "images/fig1.png",
			"image_caption":  "Figure 1: arch",
			"image_footnote": []string{"note"},
			"page_idx":       0,
		},
	})
	blocks, images, refs, err := readArtifacts(zipBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 0 || len(refs) != 0 {
		t.Fatalf("测试 zip 不应有图片字节或引用 images=%d refs=%d", len(images), len(refs))
	}
	doc := mapBlocks(blocks, images)
	if len(doc.Figures) != 1 || doc.Figures[0].Caption != "Figure 1: arch note" {
		t.Fatalf("字符串/数组图注兼容失败 %+v", doc.Figures)
	}
}

func TestMapBlocks_ChartAndCode(t *testing.T) {
	doc := mapBlocks([]contentBlock{
		{Type: "title", Text: "Method", TextLevel: 1, PageIdx: 0},
		{Type: "chart", ImgPath: "images/chart.png", ChartCaption: []string{"Figure 4: drop"}, PageIdx: 1},
		{Type: "code", CodeCaption: []string{"Algorithm 1"}, CodeBody: "step 1\nstep 2", CodeLanguage: "txt", PageIdx: 2},
	}, map[string][]byte{"chart.png": []byte("chart")})

	if len(doc.Figures) != 1 || doc.Figures[0].Caption != "Figure 4: drop" || string(doc.Figures[0].ImgData) != "chart" {
		t.Fatalf("chart 应映射为图片块 %+v", doc.Figures)
	}
	if len(doc.CodeBlocks) != 1 || doc.CodeBlocks[0].Caption != "Algorithm 1" || doc.CodeBlocks[0].SectionPath != "Method" {
		t.Fatalf("代码块映射错误 %+v", doc.CodeBlocks)
	}
}

func TestReadArtifacts_PrefersContentListV2(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	v1, err := zw.Create("paper_content_list.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v1.Write([]byte(`[{"type":"text","text":"old body","page_idx":0}]`)); err != nil {
		t.Fatal(err)
	}
	v2, err := zw.Create("paper_content_list_v2.json")
	if err != nil {
		t.Fatal(err)
	}
	v2JSON := `[[` +
		`{"type":"title","content":{"title_content":[{"type":"text","content":"V2 Intro"}],"level":1}},` +
		`{"type":"paragraph","content":{"paragraph_content":[{"type":"text","content":"v2 body"}]}},` +
		`{"type":"table","content":{"html":"<table><tr><td>A</td><td>B</td></tr><tr><td>1</td><td>2</td></tr></table>","image_source":{"path":"images/table.png"},"table_caption":[{"type":"text","content":"Table V2"}],"table_footnote":[]}},` +
		`{"type":"chart","content":{"image_source":{"path":"images/chart.png"},"chart_caption":[{"type":"text","content":"Figure V2"}],"chart_footnote":[]}},` +
		`{"type":"code","content":{"code_caption":[{"type":"text","content":"Algorithm V2"}],"code_content":[{"type":"text","content":"line 1\nline 2"}],"code_language":"txt"}},` +
		`{"type":"list","content":{"list_type":"reference_list","list_items":[{"item_content":[{"type":"text","content":"[1] V2 Ref"}]}]}}` +
		`]]`
	if _, err := v2.Write([]byte(v2JSON)); err != nil {
		t.Fatal(err)
	}
	tableImg, err := zw.Create("images/table.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tableImg.Write([]byte("tablepng")); err != nil {
		t.Fatal(err)
	}
	chartImg, err := zw.Create("images/chart.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := chartImg.Write([]byte("chartpng")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	blocks, images, refs, err := readArtifacts(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	doc := mapBlocks(blocks, images)
	doc.References = mergeReferences(doc.References, refs)

	if len(doc.Paragraphs) != 1 || doc.Paragraphs[0].Text != "v2 body" {
		t.Fatalf("应优先使用 v2 正文,得到 %+v", doc.Paragraphs)
	}
	if len(doc.Tables) != 1 || doc.Tables[0].Caption != "Table V2" || len(doc.Tables[0].Rows) != 2 {
		t.Fatalf("v2 表格映射错误 %+v", doc.Tables)
	}
	if len(doc.Figures) != 2 || string(doc.Figures[0].ImgData) != "tablepng" || string(doc.Figures[1].ImgData) != "chartpng" {
		t.Fatalf("v2 图表图片映射错误 %+v", doc.Figures)
	}
	if len(doc.CodeBlocks) != 1 || doc.CodeBlocks[0].Caption != "Algorithm V2" || doc.CodeBlocks[0].Body != "line 1\nline 2" {
		t.Fatalf("v2 代码块映射错误 %+v", doc.CodeBlocks)
	}
	if len(doc.References) != 1 || doc.References[0] != "[1] V2 Ref" {
		t.Fatalf("v2 reference_list 映射错误 %+v", doc.References)
	}
}

func TestParseArtifactDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `[` +
		`{"type":"title","text":"Intro","text_level":1,"page_idx":0},` +
		`{"type":"text","text":"body","page_idx":0},` +
		`{"type":"chart","img_path":"images/chart.png","chart_caption":["Figure 1"],"page_idx":1}` +
		`]`
	if err := os.WriteFile(filepath.Join(dir, "paper_content_list.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "layout.json"), []byte(`{"pdf_info":[{"para_blocks":[{"blocks":[{"type":"ref_text","lines":[{"spans":[{"type":"text","content":"[1] ref"}]}]}]}]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "images", "chart.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := ParseArtifactDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Paragraphs) != 1 || doc.Paragraphs[0].Text != "body" {
		t.Fatalf("归档正文解析错误 %+v", doc.Paragraphs)
	}
	if len(doc.Figures) != 1 || doc.Figures[0].Caption != "Figure 1" || string(doc.Figures[0].ImgData) != "png" {
		t.Fatalf("归档图片解析错误 %+v", doc.Figures)
	}
	if len(doc.References) != 1 || doc.References[0] != "[1] ref" {
		t.Fatalf("归档参考文献解析错误 %+v", doc.References)
	}
}
