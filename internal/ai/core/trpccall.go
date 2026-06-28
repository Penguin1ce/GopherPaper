// Package core trpccall.go 是各 trpc 链路共用的 model 调用辅助。
package core

import (
	"context"
	"fmt"
	"strings"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	trpcopenai "trpc.group/trpc-go/trpc-agent-go/model/openai"
)

// GenerateText 调 trpc model 非流式聚合为完整文本。
// 各链路均不开启流式,只读 Message.Content;若误开流式应改读 Delta,不可两者同取以免重复计数。
func GenerateText(ctx context.Context, m *trpcopenai.Model, req *trpcmodel.Request) (string, error) {
	ch, err := m.GenerateContent(ctx, req)
	if err != nil {
		return "", fmt.Errorf("agent: 模型调用失败: %w", err)
	}
	var sb strings.Builder
	for rsp := range ch {
		if rsp.Error != nil {
			return "", fmt.Errorf("agent: 响应错误: %s", rsp.Error.Message)
		}
		for _, c := range rsp.Choices {
			sb.WriteString(c.Message.Content)
		}
	}
	out := sb.String()
	if strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("agent: 模型返回空内容")
	}
	return out, nil
}

// StreamText 流式调 trpc model:开启 stream,逐增量经 onDelta 回调外发,聚合返回完整文本。
// 流式下内容在 Delta,只读 Delta 不读 Message.Content 以免重复计数。onDelta 可为 nil。
func StreamText(ctx context.Context, m *trpcopenai.Model, req *trpcmodel.Request, onDelta func(string)) (string, error) {
	req.Stream = true
	ch, err := m.GenerateContent(ctx, req)
	if err != nil {
		return "", fmt.Errorf("agent: 模型调用失败: %w", err)
	}
	var sb strings.Builder
	for rsp := range ch {
		if rsp.Error != nil {
			return "", fmt.Errorf("agent: 响应错误: %s", rsp.Error.Message)
		}
		for _, c := range rsp.Choices {
			if c.Delta.Content == "" {
				continue
			}
			sb.WriteString(c.Delta.Content)
			if onDelta != nil {
				onDelta(c.Delta.Content)
			}
		}
	}
	out := sb.String()
	if strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("agent: 模型返回空内容")
	}
	return out, nil
}
