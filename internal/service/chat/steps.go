package chat

import (
	"context"
	"strings"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/toolkit"
	"GopherPaper/pkg/constant"
)

// ExecutionStep 是前端执行过程面板的一条可持久化步骤。
type ExecutionStep struct {
	Phase  string `json:"phase"`
	Text   string `json:"text"`
	Kind   string `json:"kind,omitempty"`
	Tool   string `json:"tool,omitempty"`
	Status string `json:"status,omitempty"`
}

type executionRecorder struct {
	steps []ExecutionStep
}

func withExecutionRecorder(ctx context.Context) (context.Context, *executionRecorder) {
	rec := &executionRecorder{}
	upstream := core.StreamFrom(ctx)
	return core.WithStream(ctx, func(ev core.StreamEvent) {
		rec.Record(ev)
		if upstream != nil {
			upstream(ev)
		}
	}), rec
}

func (r *executionRecorder) Record(ev core.StreamEvent) {
	if r == nil {
		return
	}
	switch ev.Kind {
	case constant.StreamEventPlan:
		r.appendPlan(ev.Phase, ev.Delta)
	case constant.StreamEventToolCall:
		if !r.hasPlanning() {
			r.appendPlan("planning", "分析问题,制定检索策略")
		}
		text := toolStepText(ev.Tool)
		if text == "" {
			return
		}
		last := r.last()
		if last != nil && last.Phase == "action" && last.Kind == "tool" && strings.TrimSpace(last.Text) == text {
			return
		}
		r.appendTool("action", text, ev.Tool, "running")
	case constant.StreamEventToolResult:
		r.finishTool(ev.Tool)
		last := r.last()
		if last == nil || last.Phase != "reasoning" {
			r.appendPlan("reasoning", "综合检索结果,整理回答")
		}
	}
}

func (r *executionRecorder) Steps() []ExecutionStep {
	if r == nil || len(r.steps) == 0 {
		return nil
	}
	out := make([]ExecutionStep, 0, len(r.steps))
	for _, s := range r.steps {
		s.Text = strings.TrimSpace(s.Text)
		if s.Phase == "" || s.Text == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func (r *executionRecorder) appendPlan(phase, text string) {
	r.append(ExecutionStep{Phase: phase, Text: text, Kind: "plan"})
}

func (r *executionRecorder) appendTool(phase, text, tool, status string) {
	r.append(ExecutionStep{
		Phase:  phase,
		Text:   text,
		Kind:   "tool",
		Tool:   strings.TrimSpace(tool),
		Status: strings.TrimSpace(status),
	})
}

func (r *executionRecorder) append(step ExecutionStep) {
	phase := strings.TrimSpace(step.Phase)
	text := strings.TrimSpace(step.Text)
	if phase == "" && text == "" {
		return
	}
	if len(r.steps) >= constant.MaxExecutionSteps {
		return
	}
	step.Phase = phase
	step.Text = text
	if n := len(r.steps); n > 0 && r.steps[n-1].Phase == phase && r.steps[n-1].Kind != "tool" && step.Kind != "tool" {
		r.steps[n-1].Text = appendLimited(r.steps[n-1].Text, text)
		return
	}
	step.Text = trimRunes(text, constant.MaxExecutionStepTextRunes)
	r.steps = append(r.steps, step)
}

func (r *executionRecorder) finishTool(tool string) {
	if r == nil {
		return
	}
	display := toolStepText(tool)
	tool = strings.TrimSpace(tool)
	for i := len(r.steps) - 1; i >= 0; i-- {
		s := &r.steps[i]
		if s.Kind != "tool" || s.Phase != "action" {
			continue
		}
		if s.Tool == tool || strings.TrimSpace(s.Text) == display || s.Status == "running" {
			s.Status = "done"
			return
		}
	}
}

func (r *executionRecorder) hasPlanning() bool {
	for _, s := range r.steps {
		if s.Phase == "planning" || s.Phase == "replanning" {
			return true
		}
	}
	return false
}

func (r *executionRecorder) last() *ExecutionStep {
	if r == nil || len(r.steps) == 0 {
		return nil
	}
	return &r.steps[len(r.steps)-1]
}

func appendLimited(base, extra string) string {
	if extra == "" {
		return trimRunes(base, constant.MaxExecutionStepTextRunes)
	}
	return trimRunes(base+extra, constant.MaxExecutionStepTextRunes)
}

func trimRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func toolStepText(tool string) string {
	display := strings.TrimSpace(toolkit.DisplayName(strings.TrimSpace(tool)))
	switch display {
	case "search_paper", "检索论文正文":
		return "论文知识库"
	case "find_figures", "检索论文图表":
		return "图表与表格"
	default:
		return display
	}
}
