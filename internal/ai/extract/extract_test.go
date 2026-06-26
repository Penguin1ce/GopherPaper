package extract

import (
	"encoding/json"
	"strings"
	"testing"

	"GopherPaper/internal/ai/core"
)

// TestParseStructured_Fenced 验证能从带代码围栏的模型输出里解析出结构化字段。
func TestParseStructured_Fenced(t *testing.T) {
	content := "```json\n" +
		`{"title":"Attention Is All You Need","authors":["Vaswani"],"keywords":["transformer"],"methods":"self-attention","innovations":["纯注意力架构"]}` +
		"\n```"
	s, err := parseStructured(content)
	if err != nil {
		t.Fatalf("解析失败 %v", err)
	}
	if s.Title != "Attention Is All You Need" {
		t.Fatalf("title 错误 %q", s.Title)
	}
	if len(s.Authors) != 1 || s.Authors[0] != "Vaswani" {
		t.Fatalf("authors 错误 %+v", s.Authors)
	}
	if len(s.Innovations) != 1 {
		t.Fatalf("innovations 错误 %+v", s.Innovations)
	}
}

// TestParseStructured_LaTeX 验证字段里夹带 LaTeX(单反斜杠非法转义)也能解析。
func TestParseStructured_LaTeX(t *testing.T) {
	content := `{"title":"X","results":"证明了目标约束函数$\mathcal{L}_{tar}(x_t)$的样本复杂度界","keywords":["RAG"]}`
	s, err := parseStructured(content)
	if err != nil {
		t.Fatalf("解析失败 %v", err)
	}
	if !strings.Contains(s.Results, `\mathcal{L}_{tar}(x_t)`) {
		t.Fatalf("results LaTeX 丢失 %q", s.Results)
	}
	if len(s.Keywords) != 1 || s.Keywords[0] != "RAG" {
		t.Fatalf("keywords 错误 %+v", s.Keywords)
	}
}

// TestRepairJSONEscapes 验证合法转义保留、非法转义修复、字符串外反斜杠不碰。
func TestRepairJSONEscapes(t *testing.T) {
	// 合法 \n \t \uXXXX 与转义引号保留;LaTeX \mathcal \underline 修成字面反斜杠。
	in := `{"a":"换行\n制表\t引号\"星星公式\mathcal下\underline"}`
	got := repairJSONEscapes(in)
	var m map[string]string
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("修复后仍非法 %v\n%s", err, got)
	}
	want := "换行\n制表\t引号\"星星公式\\mathcal下\\underline"
	if m["a"] != want {
		t.Fatalf("修复结果错误\n got=%q\nwant=%q", m["a"], want)
	}
}

// TestStripFence_Plain 验证无围栏 JSON 也能截取。
func TestStripFence_Plain(t *testing.T) {
	got := stripFence(`前缀 {"title":"x"} 后缀`)
	if got != `{"title":"x"}` {
		t.Fatalf("截取错误 %q", got)
	}
}

// TestBuildWindows_CoversAllSections 验证长引言不会挤掉方法、结果和结论:每个章节都进了某个窗口,
// 且窗口数与单窗口大小都在上限内。
func TestBuildWindows_CoversAllSections(t *testing.T) {
	longIntro := strings.Repeat("引言背景很长。", 1200)
	doc := &core.ParsedDoc{
		Paragraphs: []core.Paragraph{
			{SectionPath: "Abstract", Text: "摘要说明关键词 GopherGNN。"},
			{SectionPath: "Introduction", Text: longIntro},
			{SectionPath: "Method", Text: "方法章节包含双塔编码器和 InfoNCE。"},
			{SectionPath: "Results", Text: "结果章节包含 MAP 提升 8.3%。"},
			{SectionPath: "Conclusion and Limitations", Text: "结论章节包含局限和未来工作。"},
		},
	}

	windows := buildWindows(doc)
	if len(windows) == 0 {
		t.Fatal("窗口为空")
	}
	if len(windows) > maxMapWindows {
		t.Fatalf("窗口数超限: %d", len(windows))
	}
	joined := strings.Join(windows, "\n")
	for _, want := range []string{
		"摘要说明关键词 GopherGNN",
		"方法章节包含双塔编码器",
		"结果章节包含 MAP 提升",
		"结论章节包含局限",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("窗口缺少 %q", want)
		}
	}
	for i, w := range windows {
		if got := len([]rune(w)); got > mapWindowRunes {
			t.Fatalf("窗口 %d 超长: %d", i, got)
		}
	}
}

func TestReferenceTitle(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{
			in:   `[1] W. Dai, "b-money," http://www.weidai.com/bmoney.txt, 1998.`,
			want: "b-money,",
		},
		{
			in:   `[3] S. Haber, W.S. Stornetta, "How to time-stamp a digital document," In Journal of Cryptology, 1991.`,
			want: "How to time-stamp a digital document,",
		},
		{
			in:   `[8] W. Feller, An introduction to probability theory and its applications, 1957.`,
			want: "W. Feller, An introduction to probability theory and its applications, 1957.",
		},
	}
	for _, tc := range cases {
		if got := referenceTitle(tc.in); got != tc.want {
			t.Fatalf("referenceTitle(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestMergePartials 验证确定性合并:列表并集去重、题目取首个非空、文本取最长。
func TestMergePartials(t *testing.T) {
	got := mergePartials([]*core.PaperStructured{
		{Title: "SafeVLA", Authors: []string{"Zhang", "Ji"}, Methods: "短", Keywords: []string{"VLA"}},
		{Title: "", Authors: []string{"Ji", "Lei"}, Methods: "更长的方法描述", Keywords: []string{"VLA", "safety"}},
	})
	if got.Title != "SafeVLA" {
		t.Fatalf("title 错误 %q", got.Title)
	}
	if got.Methods != "更长的方法描述" {
		t.Fatalf("methods 应取最长 %q", got.Methods)
	}
	if strings.Join(got.Authors, ",") != "Zhang,Ji,Lei" {
		t.Fatalf("authors 并集去重错误 %+v", got.Authors)
	}
	if strings.Join(got.Keywords, ",") != "VLA,safety" {
		t.Fatalf("keywords 并集去重错误 %+v", got.Keywords)
	}
}

func TestMergePartials_YearVenue(t *testing.T) {
	// year 取首个非 0,venue 取首个非空,缺失片段不应覆盖已有值。
	got := mergePartials([]*core.PaperStructured{
		{Title: "A", PublishYear: 0, Venue: ""},
		{Title: "A", PublishYear: 2023, Venue: "NeurIPS"},
		{Title: "A", PublishYear: 2022, Venue: "ICML"},
	})
	if got.PublishYear != 2023 {
		t.Fatalf("publish_year 应取首个非 0 = 2023, got %d", got.PublishYear)
	}
	if got.Venue != "NeurIPS" {
		t.Fatalf("venue 应取首个非空 = NeurIPS, got %q", got.Venue)
	}
}

func TestBackfillStructured_YearVenue(t *testing.T) {
	// primary 缺 year/venue 时用 fallback 补,已有则保留。
	primary := &core.PaperStructured{Title: "A", PublishYear: 0, Venue: ""}
	fallback := &core.PaperStructured{Title: "A", PublishYear: 2021, Venue: "CVPR"}
	got := backfillStructured(primary, fallback)
	if got.PublishYear != 2021 || got.Venue != "CVPR" {
		t.Fatalf("回填错误: year=%d venue=%q", got.PublishYear, got.Venue)
	}
}
