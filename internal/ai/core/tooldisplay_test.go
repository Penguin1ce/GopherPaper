package core

import (
	"testing"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

func TestToolDisplayTrackerSkillLoad(t *testing.T) {
	tracker := NewToolDisplayTracker()
	label := tracker.CallLabel(trpcmodel.ToolCall{
		ID: "call-1",
		Function: trpcmodel.FunctionDefinitionParam{
			Name:      "skill_load",
			Arguments: []byte(`{"skill":"find-papers"}`),
		},
	})
	if label != "Skill：find-papers" {
		t.Fatalf("unexpected call label: %q", label)
	}
	if got := tracker.ResultLabel("call-1", "skill_load"); got != label {
		t.Fatalf("result label should match call label, got %q want %q", got, label)
	}
}

func TestToolDisplayTrackerKeepsNormalToolName(t *testing.T) {
	tracker := NewToolDisplayTracker()
	label := tracker.CallLabel(trpcmodel.ToolCall{
		ID: "call-1",
		Function: trpcmodel.FunctionDefinitionParam{
			Name:      "search_arxiv",
			Arguments: []byte(`{"query":"rag"}`),
		},
	})
	if label != "search_arxiv" {
		t.Fatalf("normal tool label changed: %q", label)
	}
	if got := tracker.ResultLabel("call-1", "search_arxiv"); got != "search_arxiv" {
		t.Fatalf("normal result label changed: %q", got)
	}
}

func TestToolDisplayTrackerParsesEncodedArguments(t *testing.T) {
	tracker := NewToolDisplayTracker()
	label := tracker.CallLabel(trpcmodel.ToolCall{
		ID: "call-1",
		Function: trpcmodel.FunctionDefinitionParam{
			Name:      "skill_select_docs",
			Arguments: []byte(`"{\"skill\":\"literature-review\"}"`),
		},
	})
	if label != "Skill 文档：literature-review" {
		t.Fatalf("unexpected encoded args label: %q", label)
	}
}
