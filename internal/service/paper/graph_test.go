package paper

import (
	"strings"
	"testing"

	"GopherPaper/internal/ai/core"
)

func TestGraphSemanticText(t *testing.T) {
	text := graphSemanticText(&core.PaperStructured{
		Title:             "Retrieval-Augmented Generation for Scientific Papers",
		Abstract:          "We study retrieval augmented paper question answering.",
		Authors:           []string{"Alice"},
		Affiliations:      []string{"Example University"},
		PublishYear:       2024,
		Keywords:          []string{"RAG", "scientific QA", "knowledge graph"},
		ResearchQuestions: []string{"How to ground paper answers with citations?"},
		Methods:           "A hybrid dense retrieval and graph expansion method.",
		Innovations:       []string{"Citation-aware evidence aggregation"},
	})

	for _, want := range []string{
		"标题: Retrieval-Augmented Generation for Scientific Papers",
		"摘要: We study retrieval augmented paper question answering.",
		"关键词: RAG；scientific QA；knowledge graph",
		"研究问题: How to ground paper answers with citations?",
		"方法: A hybrid dense retrieval and graph expansion method.",
		"创新点: Citation-aware evidence aggregation",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("graphSemanticText 缺少 %q, got:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"Alice", "Example University", "2024"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("graphSemanticText 不应包含非主题字段 %q, got:\n%s", unwanted, text)
		}
	}
}

func TestGraphSemanticTextTruncatesLongMethods(t *testing.T) {
	text := graphSemanticText(&core.PaperStructured{
		Title:   "Long Method Paper",
		Methods: strings.Repeat("方", 1300),
	})
	methodLine := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "方法: ") {
			methodLine = strings.TrimPrefix(line, "方法: ")
			break
		}
	}
	if strings.Count(methodLine, "方") != 1200 {
		t.Fatalf("方法字段应截断到 1200 字符, got %d", strings.Count(methodLine, "方"))
	}
}

func TestGraphSemanticTextEmpty(t *testing.T) {
	if got := graphSemanticText(nil); got != "" {
		t.Fatalf("nil 输入应返回空串, got %q", got)
	}
	if got := graphSemanticText(&core.PaperStructured{}); got != "" {
		t.Fatalf("空结构应返回空串, got %q", got)
	}
}
