// report_trpc.go 是 report 链路:检索 + chat 模型按报告类型生成研读报告。
// 检索与 model 解耦(复用 chat_pipeline 的 retriever),按报告类型选 prompt,Meta 标 report_type。
package report_pipeline

import (
	"context"
	"fmt"
	"strings"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	trpcopenai "trpc.group/trpc-go/trpc-agent-go/model/openai"

	"GopherPaper/internal/agent"
	"GopherPaper/internal/agent/chat_pipeline"
	"GopherPaper/internal/config"
	"GopherPaper/pkg/constant"
)

// GenerateReportTRPC 围绕某篇论文按类型生成研读报告。
func GenerateReportTRPC(ctx context.Context, m *trpcopenai.Model, mc config.ModelConfig, in *agent.ReportInput) (*agent.Reply, error) {
	query := in.Query
	if strings.TrimSpace(query) == "" {
		query = string(in.ReportType)
	}
	docs, err := chat_pipeline.RetrieveForPaper(ctx, query, in.OwnerID, in.PaperID)
	if err != nil {
		docs = nil
	}
	// compare 类型再补一轮跨库检索,召回同类文献做对比。
	if in.ReportType == constant.ReportCompare {
		if more, err := chat_pipeline.RetrieveVisible(ctx, query, in.OwnerID); err == nil {
			docs = append(docs, more...)
		}
	}

	sysPrompt := strings.ReplaceAll(constant.ReportPromptFor(in.ReportType), "{context}", formatDocs(docs))
	req := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(sysPrompt),
			trpcmodel.NewUserMessage("请基于以上论文片段生成报告。"),
		},
	}
	if mc.MaxTokens > 0 {
		req.MaxTokens = &mc.MaxTokens
	}
	if mc.ReasoningEffort != "" {
		req.ReasoningEffort = &mc.ReasoningEffort
	}

	content, err := agent.GenerateText(ctx, m, req)
	if err != nil {
		return nil, err
	}
	return &agent.Reply{
		Content: content,
		Meta:    map[string]any{"report_type": string(in.ReportType)},
	}, nil
}

// formatDocs 把召回片段拼成带页码出处的报告 context,无召回时给模型明确占位。
func formatDocs(docs []*chat_pipeline.Doc) string {
	if len(docs) == 0 {
		return "无相关片段"
	}
	var b strings.Builder
	for i, d := range docs {
		ref := chat_pipeline.ReferenceFromDocument(d)
		fmt.Fprintf(&b, "[%d] 出处: 第 %d 页\n%s\n", i+1, ref.PageNo, d.Content)
	}
	return b.String()
}
