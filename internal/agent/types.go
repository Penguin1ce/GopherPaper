// Package agent 定义各下游 agent 共享的输入输出契约。
package agent

import "GopherCPP/pkg/constant"

// Intent 是意图识别小模型的结构化输出。
type Intent struct {
	Type  constant.IntentType `json:"type"`
	Slots map[string]string   `json:"slots"` // 抽取出的参数，如知识点、数量、难度
}

// Request 是一次助教请求的原始输入。学生身份经 context 透传，不放这里。
type Request struct {
	Query string `json:"query"`
}

// AgentInput 是 Branch 之后分发给具体 agent 的统一输入。
type AgentInput struct {
	Query  string
	Intent Intent
}

// Reply 是所有 agent 的统一输出。
type Reply struct {
	Intent  constant.IntentType `json:"intent"`
	Content string              `json:"content"`
	Meta    map[string]any      `json:"meta,omitempty"` // agent 特有的结构化数据
}
