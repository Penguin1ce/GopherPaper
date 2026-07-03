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

func TestExtractPromptsAllowKeywordFallback(t *testing.T) {
	for name, prompt := range map[string]string{
		"extract":        ExtractPrompt,
		"extract-reduce": ExtractReducePrompt,
	} {
		if !strings.Contains(prompt, "3-8 个短主题词") {
			t.Fatalf("%s prompt 缺少无显式关键词时的主题词兜底", name)
		}
		if !strings.Contains(prompt, "完全无依据才留空数组") {
			t.Fatalf("%s prompt 缺少关键词留空边界", name)
		}
	}
}

func TestIntentPromptRoutesPaperResourcesToSummary(t *testing.T) {
	for _, want := range []string{"GitHub", "代码仓库", "项目主页", "arXiv", "补充材料", "数据集地址"} {
		if !strings.Contains(IntentPrompt, want) {
			t.Fatalf("IntentPrompt 缺少论文资源分类提示: %s", want)
		}
	}
	if !strings.Contains(IntentPrompt, "只有在问题不指向当前论文") {
		t.Fatal("IntentPrompt 缺少 chitchat 与当前论文资源的边界提示")
	}
}
