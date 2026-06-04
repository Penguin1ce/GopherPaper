package extract_pipeline

import "testing"

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
