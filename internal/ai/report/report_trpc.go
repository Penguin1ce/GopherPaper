// report_trpc.go 是 report 链路:检索 + chat 模型按报告类型生成研读报告。
// 检索与 model 解耦，复用 chat 检索器，按报告类型选 prompt。
package report

import (
	"context"
	"strings"

	"GopherPaper/internal/ai/agentrt"
	"GopherPaper/internal/ai/chat"
	"GopherPaper/internal/ai/core"
	"GopherPaper/pkg/constant"
)

// GenerateReportTRPC 围绕某篇论文按类型生成研读报告,经带工具 chat agent 生成。
func GenerateReportTRPC(ctx context.Context, in *core.ReportInput) (*core.Reply, error) {
	query := in.Query
	if strings.TrimSpace(query) == "" {
		query = string(in.ReportType)
	}
	docs, err := chat.RetrieveForPaper(ctx, query, in.OwnerID, in.PaperID)
	if err != nil {
		docs = nil
	}
	// compare 类型再补一轮跨库检索,召回同类文献做对比。
	if in.ReportType == constant.ReportCompare {
		if more, err := chat.RetrieveVisible(ctx, query, in.OwnerID); err == nil {
			docs = append(docs, more...)
		}
	}

	sysPrompt := strings.ReplaceAll(constant.ReportPromptFor(in.ReportType), "{context}", chat.FormatDocs(docs))
	content, err := agentrt.Generate(ctx, in.OwnerID, sysPrompt, nil, "请基于以上论文片段生成报告。")
	if err != nil {
		return nil, err
	}
	return &core.Reply{
		Content: content,
		Meta:    map[string]any{"report_type": string(in.ReportType)},
	}, nil
}
