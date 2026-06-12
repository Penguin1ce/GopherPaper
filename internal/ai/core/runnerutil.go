// Package core runnerutil.go 是各 agent runner 共用的事件聚合与生成参数辅助。
package core

import (
	"fmt"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/event"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	"GopherPaper/internal/config"
)

// CollectEvents 聚合 runner 事件流为完整答案,跳过工具结果与 runner 收尾事件,避免混入或重复计数。
func CollectEvents(ch <-chan *event.Event) (string, error) {
	var sb strings.Builder
	for ev := range ch {
		if ev.Error != nil {
			return "", fmt.Errorf("agent: %s", ev.Error.Message)
		}
		if ev.Object == trpcmodel.ObjectTypeToolResponse || ev.IsRunnerCompletion() {
			continue
		}
		for _, c := range ev.Choices {
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
