// Package core runnerutil.go 是各 agent runner 共用的事件聚合与生成参数辅助。
package core

import (
	"context"
	"fmt"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/event"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	"GopherPaper/internal/config"
	"GopherPaper/pkg/constant"
)

// CollectEvents 聚合 runner 事件流为完整答案,跳过工具结果与 runner 收尾事件,避免混入或重复计数。
// ctx 带 StreamHandler 时把工具调用与文本增量实时外发;流式下增量在 Delta、完整文本仍落
// 收尾的非 partial 事件,故聚合结果与外发互不重复。
func CollectEvents(ctx context.Context, ch <-chan *event.Event) (string, error) {
	emit := StreamFrom(ctx)
	var sb strings.Builder
	for ev := range ch {
		if ev.Error != nil {
			return "", fmt.Errorf("agent: %s", ev.Error.Message)
		}
		if ev.Object == trpcmodel.ObjectTypeToolResponse {
			if emit != nil {
				for _, c := range ev.Choices {
					if c.Message.ToolName != "" {
						emit(StreamEvent{Kind: constant.StreamEventToolResult, Tool: c.Message.ToolName})
					}
				}
			}
			continue
		}
		if ev.IsRunnerCompletion() {
			continue
		}
		for _, c := range ev.Choices {
			if emit != nil {
				if ev.IsPartial && c.Delta.Content != "" {
					emit(StreamEvent{Kind: constant.StreamEventDelta, Delta: c.Delta.Content})
				}
				if !ev.IsPartial {
					for _, tc := range c.Message.ToolCalls {
						emit(StreamEvent{Kind: constant.StreamEventToolCall, Tool: tc.Function.Name})
					}
				}
			}
			sb.WriteString(c.Message.Content)
		}
	}
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return "", fmt.Errorf("agent: 模型返回空内容")
	}
	return out, nil
}

// GenConfig 把 ModelConfig 的生成参数映射到 trpc 的 GenerationConfig。
func GenConfig(mc config.ModelConfig) trpcmodel.GenerationConfig {
	var gc trpcmodel.GenerationConfig
	if mc.MaxTokens > 0 {
		gc.MaxTokens = &mc.MaxTokens
	}
	if mc.ReasoningEffort != "" {
		gc.ReasoningEffort = &mc.ReasoningEffort
	}
	return gc
}
