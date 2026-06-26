package core

import (
	"encoding/json"
	"strings"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
)

// ToolDisplayTracker keeps tool_call and tool_result labels aligned within one
// streamed runner pass. It is mainly needed for skill_load: the call arguments
// contain the actual skill name, while the result only carries the generic tool
// name.
type ToolDisplayTracker struct {
	byID map[string]string
}

// NewToolDisplayTracker creates a per-run tracker for streamed tool labels.
func NewToolDisplayTracker() *ToolDisplayTracker {
	return &ToolDisplayTracker{byID: map[string]string{}}
}

// CallLabel returns the display label for a tool call and remembers it by call id.
func (t *ToolDisplayTracker) CallLabel(tc trpcmodel.ToolCall) string {
	raw := strings.TrimSpace(tc.Function.Name)
	label := skillToolLabel(raw, tc.Function.Arguments)
	if label == "" {
		label = raw
	}
	if t != nil && tc.ID != "" && label != "" {
		t.byID[tc.ID] = label
	}
	return label
}

// ResultLabel returns the matching display label for a tool result.
func (t *ToolDisplayTracker) ResultLabel(toolID, toolName string) string {
	if t != nil && toolID != "" {
		if label := t.byID[toolID]; label != "" {
			delete(t.byID, toolID)
			return label
		}
	}
	raw := strings.TrimSpace(toolName)
	if label := skillToolLabel(raw, nil); label != "" {
		return label
	}
	return raw
}

type skillToolArgs struct {
	Skill string `json:"skill"`
}

func skillToolLabel(toolName string, args []byte) string {
	prefix := ""
	switch strings.TrimSpace(toolName) {
	case "skill_load":
		prefix = "Skill"
	case "skill_list_docs":
		prefix = "Skill 文档"
	case "skill_select_docs":
		prefix = "Skill 文档"
	case "skill_run":
		prefix = "Skill 执行"
	default:
		return ""
	}
	if skill := skillNameFromArgs(args); skill != "" {
		return prefix + "：" + skill
	}
	return prefix
}

func skillNameFromArgs(args []byte) string {
	raw := strings.TrimSpace(string(args))
	if raw == "" {
		return ""
	}
	if skill := parseSkillName([]byte(raw)); skill != "" {
		return skill
	}
	var encoded string
	if err := json.Unmarshal([]byte(raw), &encoded); err == nil {
		return parseSkillName([]byte(encoded))
	}
	return ""
}

func parseSkillName(args []byte) string {
	var in skillToolArgs
	if err := json.Unmarshal(args, &in); err != nil {
		return ""
	}
	return strings.TrimSpace(in.Skill)
}
