package gopher

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/retrieval"
	"GopherPaper/pkg/constant"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

// 报告 prompt 必须带 {focus} 占位符,Generate 据此按报告类型注入聚焦点。
func TestReportPromptHasFocusPlaceholder(t *testing.T) {
	if !strings.Contains(constant.GopherReportPrompt, "{focus}") {
		t.Fatal("GopherReportPrompt 缺少 {focus} 占位符,Generate 无法注入报告聚焦点")
	}
	// 注入后不应残留占位符,且聚焦点文本在位。
	out := reportQuery(constant.ReportMethodFocus)
	if strings.Contains(out, "{focus}") || !strings.Contains(out, constant.ReportMethodFocus) {
		t.Fatalf("focus 注入异常: %q", out)
	}
	// 共享用户消息不得下达角色顺序指令,否则挂 planner 的 researcher 会把写作评审规划进自己任务。
	for _, role := range []string{"researcher", "writer", "reviewer"} {
		if strings.Contains(out, role) {
			t.Fatalf("共享任务简报不应出现 %s 角色指令: %q", role, out)
		}
	}
}

func TestGopherChainPromptsSeparateRoles(t *testing.T) {
	for name, prompt := range map[string]string{
		"researcher": constant.GopherResearcherPrompt,
		"writer":     constant.GopherWriterPrompt,
		"reviewer":   constant.GopherReviewerPrompt,
	} {
		if strings.TrimSpace(prompt) == "" {
			t.Fatalf("%s prompt 不能为空", name)
		}
	}
	if !strings.Contains(constant.GopherResearcherPrompt, "证据笔记") {
		t.Fatal("researcher prompt 应明确只产证据笔记")
	}
	if !strings.Contains(constant.GopherWriterPrompt, "不要新增未经证据支撑") {
		t.Fatal("writer prompt 应约束不得新增无证据事实")
	}
	if !strings.Contains(constant.GopherReviewerPrompt, "最终只输出修订后的报告正文") {
		t.Fatal("reviewer prompt 应约束最终只输出报告正文")
	}
}

func TestEmitReportPhase(t *testing.T) {
	var events []core.StreamEvent
	ctx := core.WithStream(context.Background(), func(ev core.StreamEvent) {
		events = append(events, ev)
	})
	emitReportPhase(ctx, constant.ReportPhaseWriting, "写报告:正在整理证据。")
	if len(events) != 1 {
		t.Fatalf("阶段事件数量 = %d, want 1", len(events))
	}
	ev := events[0]
	if ev.Kind != constant.StreamEventPlan || ev.Phase != constant.ReportPhaseWriting {
		t.Fatalf("阶段事件不符: %+v", ev)
	}
	if !strings.Contains(ev.Delta, "写报告") {
		t.Fatalf("阶段文案缺少写报告提示: %q", ev.Delta)
	}
}

func TestResearcherEventContent(t *testing.T) {
	ev := &event.Event{Response: &trpcmodel.Response{
		Choices: []trpcmodel.Choice{{Message: trpcmodel.Message{Content: "  证据笔记  "}}},
	}}
	if got := researcherEventContent(ev); got != "证据笔记" {
		t.Fatalf("researcherEventContent = %q, want 证据笔记", got)
	}
	toolEv := &event.Event{Response: &trpcmodel.Response{
		Object:  trpcmodel.ObjectTypeToolResponse,
		Choices: []trpcmodel.Choice{{Message: trpcmodel.Message{Content: "工具结果"}}},
	}}
	if got := researcherEventContent(toolEv); got != "" {
		t.Fatalf("工具响应不应作为 researcher 最终内容: %q", got)
	}
}

func TestResearcherEventContentFromStateDelta(t *testing.T) {
	raw, err := json.Marshal("state 证据笔记")
	if err != nil {
		t.Fatal(err)
	}
	ev := &event.Event{
		Response:   &trpcmodel.Response{},
		StateDelta: map[string][]byte{graph.StateKeyLastResponse: raw},
	}
	if got := researcherEventContent(ev); got != "state 证据笔记" {
		t.Fatalf("researcherEventContent state = %q", got)
	}
}

func TestFallbackReportQueriesByType(t *testing.T) {
	if got := fallbackReportQueries(constant.ReportQuickRead); len(got) < 5 {
		t.Fatalf("quickread 兜底检索主题过少: %v", got)
	}
	result := strings.Join(fallbackReportQueries(constant.ReportResult), " ")
	for _, want := range []string{"主实验", "消融实验", "失败案例"} {
		if !strings.Contains(result, want) {
			t.Fatalf("result 兜底检索主题缺少 %q: %s", want, result)
		}
	}
}

func TestExtractSVG(t *testing.T) {
	in := "说明文字\n```svg\n<svg viewBox=\"0 0 680 760\"><text>思路图</text></svg>\n```"
	got := extractSVG(in)
	want := `<svg viewBox="0 0 680 760"><text>思路图</text></svg>`
	if got != want {
		t.Fatalf("extractSVG = %q, want %q", got, want)
	}
}

func TestFallbackFlowQueriesCoverMainChain(t *testing.T) {
	got := strings.Join(fallbackFlowQueries(), " ")
	for _, want := range []string{"研究问题", "现有方法不足", "核心思路", "方法流程", "关键结果", "结论"} {
		if !strings.Contains(got, want) {
			t.Fatalf("思路图兜底检索主题缺少 %q: %s", want, got)
		}
	}
}

func TestAppendUniqueDocsLimitsAndDedupes(t *testing.T) {
	seen := map[string]struct{}{"a": {}}
	out := []*retrieval.Doc{{ID: "a"}}
	docs := []*retrieval.Doc{
		{ID: "a"},
		{ID: "b"},
		nil,
		{ID: "c"},
		{ID: "d"},
	}
	out = appendUniqueDocs(out, seen, docs, 2, 3)
	if len(out) != 3 {
		t.Fatalf("去重追加数量 = %d, want 3", len(out))
	}
	if out[1].ID != "b" || out[2].ID != "c" {
		t.Fatalf("追加顺序或去重异常: %+v", out)
	}
	if _, ok := seen["d"]; ok {
		t.Fatalf("达到 maxAdd 后不应继续标记后续 doc")
	}
}

// 空 userID 必须报错,不能懒建出无主 runner。
func TestRunnerForUserRejectsEmpty(t *testing.T) {
	if _, err := runnerForUser(""); err == nil {
		t.Fatal("runnerForUser(\"\") 应返回错误")
	}
}

// 模型跑飞重写第二份报告时,从第二个一级标题处截断,只保留第一份。
// 标题之前残留的思维链自语/垃圾串交由采样参数治理,不在此正则范围。
func TestSanitizeReportCutsDuplicate(t *testing.T) {
	in := "# 报告一\n\n## 背景\n正文内容。\n\n# 报告二\n\n## 背景\n第二份正文。"
	out := sanitizeReport(in)
	if strings.Contains(out, "报告二") || strings.Contains(out, "第二份正文") {
		t.Fatalf("第二份报告未截掉: %q", out)
	}
	if !strings.HasPrefix(out, "# 报告一") || !strings.Contains(out, "正文内容") {
		t.Fatalf("第一份报告被误伤: %q", out)
	}
}

// 单份报告(只有一个一级标题)原样保留,不得误删。
func TestSanitizeReportKeepsSingle(t *testing.T) {
	in := "# 报告\n\n## 方法\n正文。"
	if out := sanitizeReport(in); out != in {
		t.Fatalf("单份报告被改动: %q", out)
	}
}

func TestSanitizeReportStripsPlannerTags(t *testing.T) {
	in := "/*PLANNING*/先检查。/*FINAL_ANSWER*/# 报告\n\n## 方法\n正文。"
	out := sanitizeReport(in)
	if strings.Contains(out, "PLANNING") || strings.Contains(out, "FINAL_ANSWER") || strings.Contains(out, "先检查") {
		t.Fatalf("planner 标签或过程文字未清理: %q", out)
	}
	if !strings.HasPrefix(out, "# 报告") || !strings.Contains(out, "正文") {
		t.Fatalf("最终报告被误伤: %q", out)
	}
}
