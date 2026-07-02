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

func TestPaperAnswerPromptsUseCitationTagAndTreatMaterialsAsData(t *testing.T) {
	cases := map[string]string{
		"summary":          SummaryPrompt,
		"method":           MethodPrompt,
		"agentic-summary":  AgenticRAGPromptFor(IntentSummary),
		"agentic-method":   AgenticRAGPromptFor(IntentMethod),
		"maodie":           MaodiePrompt,
		"gopher-report":    GopherReportPrompt,
		"gopher-fallback":  GopherFallbackReportPrompt,
		"gopher-flow":      GopherFlowPrompt,
		"gopher-flow-fb":   GopherFallbackFlowPrompt,
		"pioneer":          PioneerInstruction,
		"extract":          ExtractPrompt,
		"extract-reduce":   ExtractReducePrompt,
		"translate":        TranslatePrompt,
		"flow-skeleton":    PaperFlowSkeletonPrompt,
		"flow-node-detail": PaperFlowNodeDetailPrompt,
	}
	for name, prompt := range cases {
		if !strings.Contains(prompt, "待分析资料") && !strings.Contains(prompt, "待翻译资料") {
			t.Fatalf("%s prompt 缺少资料非指令约束", name)
		}
	}
	for name, prompt := range map[string]string{
		"summary":         SummaryPrompt,
		"method":          MethodPrompt,
		"agentic-summary": AgenticRAGPromptFor(IntentSummary),
		"agentic-method":  AgenticRAGPromptFor(IntentMethod),
		"gopher-report":   GopherReportPrompt,
		"gopher-research": GopherResearcherPrompt,
		"gopher-fallback": GopherFallbackReportPrompt,
	} {
		if !strings.Contains(prompt, "citation_tag") {
			t.Fatalf("%s prompt 缺少 citation_tag 复制约束", name)
		}
	}
}
