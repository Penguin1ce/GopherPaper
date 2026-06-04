// Package report_pipeline 实现研读报告 agent，围绕某篇论文按报告类型生成 Markdown。
// 由前端按钮带 ReportType 显式触发，不经意图分类器。检索片段作为上下文喂给模型。
package report_pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"GopherPaper/internal/agent"
	"GopherPaper/internal/agent/chat_pipeline"
	"GopherPaper/pkg/constant"
)

// Build 构造研读报告 agent 子图。
func Build(chatModel model.BaseChatModel) (*compose.Graph[*agent.ReportInput, *agent.Reply], error) {
	g := compose.NewGraph[*agent.ReportInput, *agent.Reply](
		compose.WithGenLocalState(func(context.Context) *reportState { return &reportState{} }),
	)

	prepare := compose.InvokableLambda(func(ctx context.Context, in *agent.ReportInput) ([]*schema.Message, error) {
		query := in.Query
		if strings.TrimSpace(query) == "" {
			query = string(in.ReportType)
		}
		docs, err := chat_pipeline.RetrieveForPaper(ctx, query, in.OwnerID, in.PaperID)
		if err != nil {
			docs = nil
		}
		// compare 类型再补一轮跨库检索，召回同类文献做对比。
		if in.ReportType == constant.ReportCompare {
			if more, err := chat_pipeline.RetrieveVisible(ctx, query, in.OwnerID); err == nil {
				docs = append(docs, more...)
			}
		}
		sysPrompt := strings.ReplaceAll(constant.ReportPromptFor(in.ReportType), "{context}", formatDocs(docs))
		return []*schema.Message{
			schema.SystemMessage(sysPrompt),
			schema.UserMessage("请基于以上论文片段生成报告。"),
		}, nil
	})

	toReply := compose.InvokableLambda(func(_ context.Context, msg *schema.Message) (*agent.Reply, error) {
		return &agent.Reply{Content: msg.Content}, nil
	})
	tagType := compose.WithStatePostHandler(func(_ context.Context, out *agent.Reply, s *reportState) (*agent.Reply, error) {
		if out.Meta == nil {
			out.Meta = map[string]any{}
		}
		out.Meta["report_type"] = string(s.ReportType)
		return out, nil
	})
	keepType := compose.WithStatePreHandler(func(_ context.Context, in *agent.ReportInput, s *reportState) (*agent.ReportInput, error) {
		s.ReportType = in.ReportType
		return in, nil
	})

	_ = g.AddLambdaNode("prepare", prepare, keepType)
	_ = g.AddChatModelNode("model", chatModel)
	_ = g.AddLambdaNode("reply", toReply, tagType)
	_ = g.AddEdge(compose.START, "prepare")
	_ = g.AddEdge("prepare", "model")
	_ = g.AddEdge("model", "reply")
	_ = g.AddEdge("reply", compose.END)
	return g, nil
}

// reportState 把报告类型透传到 reply 的 Meta。
type reportState struct {
	ReportType constant.ReportType
}

func formatDocs(docs []*schema.Document) string {
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
