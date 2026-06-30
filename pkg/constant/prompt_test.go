package constant

import (
	"strings"
	"testing"
)

func TestPaperAnswerPromptsRequireInlineSourceTags(t *testing.T) {
	cases := map[string]string{
		"summary":         SummaryPrompt,
		"method":          MethodPrompt,
		"agentic-summary": AgenticRAGPromptFor(IntentSummary),
		"agentic-method":  AgenticRAGPromptFor(IntentMethod),
		"gopher-report":   GopherReportPrompt,
		"gopher-research": GopherResearcherPrompt,
		"gopher-writer":   GopherWriterPrompt,
		"gopher-reviewer": GopherReviewerPrompt,
	}
	for name, prompt := range cases {
		if !strings.Contains(prompt, "[[原文:") {
			t.Fatalf("%s prompt 缺少正文内联出处标签约束", name)
		}
	}
}
