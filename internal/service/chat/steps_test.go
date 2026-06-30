package chat

import (
	"strings"
	"testing"

	"GopherPaper/internal/ai/core"
	"GopherPaper/pkg/constant"
)

func TestExecutionRecorderSynthesizesToolSteps(t *testing.T) {
	rec := &executionRecorder{}
	rec.Record(core.StreamEvent{Kind: constant.StreamEventToolCall, Tool: "search_paper"})
	rec.Record(core.StreamEvent{Kind: constant.StreamEventToolResult, Tool: "search_paper"})

	steps := rec.Steps()
	if len(steps) != 3 {
		t.Fatalf("steps len = %d, want 3: %+v", len(steps), steps)
	}
	if steps[0].Phase != "planning" {
		t.Fatalf("首步应为 planning: %+v", steps[0])
	}
	if steps[1].Phase != "action" || steps[1].Text != "论文知识库" {
		t.Fatalf("工具调用步骤不符: %+v", steps[1])
	}
	if steps[2].Phase != "reasoning" {
		t.Fatalf("工具返回后应进入 reasoning: %+v", steps[2])
	}
}

func TestExecutionRecorderAppendsPlanChunks(t *testing.T) {
	rec := &executionRecorder{}
	rec.Record(core.StreamEvent{Kind: constant.StreamEventPlan, Phase: "planning", Delta: "先分析"})
	rec.Record(core.StreamEvent{Kind: constant.StreamEventPlan, Phase: "planning", Delta: ",再检索"})
	rec.Record(core.StreamEvent{Kind: constant.StreamEventPlan, Phase: "action", Delta: "查图表"})

	steps := rec.Steps()
	if len(steps) != 2 {
		t.Fatalf("steps len = %d, want 2: %+v", len(steps), steps)
	}
	if got := steps[0].Text; !strings.Contains(got, "先分析,再检索") {
		t.Fatalf("同 phase 增量未合并: %q", got)
	}
	if steps[1].Phase != "action" || steps[1].Text != "查图表" {
		t.Fatalf("action 步骤不符: %+v", steps[1])
	}
}
