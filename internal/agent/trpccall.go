// trpccall.go 是各 trpc 垂直切片共用的 model 调用辅助(迁移期)。
package agent

import (
	"context"
	"fmt"
	"strings"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	trpcopenai "trpc.group/trpc-go/trpc-agent-go/model/openai"
)

// GenerateText 调 trpc model 非流式聚合为完整文本。
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
			sb.WriteString(c.Delta.Content)
		}
	}
	return sb.String(), nil
}
