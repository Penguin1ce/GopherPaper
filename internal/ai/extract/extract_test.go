package extract

import (
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
