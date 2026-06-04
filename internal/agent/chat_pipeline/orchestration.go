// Package chat_pipeline 实现论文问答 agent，并内聚多租户检索与出处召回。
// fact/summary/method 三个问答子类共用本 agent，按子类选 prompt，检索与引用集中在本包。
// ctx 经 agent.WithPaperID 注入论文 ID 时围绕该论文检索，否则跨可见库检索。
package chat_pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"GopherPaper/internal/agent"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
)

// ragState 把答疑子类透传到 reply，用于标注响应意图。
type ragState struct {
	Intent  constant.IntentType
	Sources []Reference
}

// Build 构造 RAG agent 子图。
func Build(chatModel model.BaseChatModel) (*compose.Graph[*agent.AgentInput, *agent.Reply], error) {
	g := compose.NewGraph[*agent.AgentInput, *agent.Reply](
		compose.WithGenLocalState(func(context.Context) *ragState { return &ragState{} }),
	)

	build := compose.InvokableLambda(func(ctx context.Context, in *agent.AgentInput) ([]*schema.Message, error) {
		sysPrompt := strings.ReplaceAll(constant.RAGPromptFor(in.Intent.Type), "{context}", in.Intent.Slots["rag_context"])
		return []*schema.Message{
			schema.SystemMessage(sysPrompt),
			schema.UserMessage(in.Query),
		}, nil
	})
	saveContext := compose.WithStatePreHandler(func(ctx context.Context, in *agent.AgentInput, s *ragState) (*agent.AgentInput, error) {
		s.Intent = in.Intent.Type
		docs, err := RetrieveForPaper(ctx, in.Query, tenant.MustStudentID(ctx), agent.PaperIDFrom(ctx))
		if err != nil {
			docs = nil
		}
		s.Sources = References(docs)
		if in.Intent.Slots == nil {
			in.Intent.Slots = map[string]string{}
		}
		in.Intent.Slots["rag_context"] = formatDocs(docs)
		return in, nil
	})

	toReply := compose.InvokableLambda(func(_ context.Context, msg *schema.Message) (*agent.Reply, error) {
		return &agent.Reply{Content: msg.Content}, nil
	})
	fillIntent := compose.WithStatePostHandler(func(_ context.Context, out *agent.Reply, s *ragState) (*agent.Reply, error) {
		out.Intent = s.Intent
		if len(s.Sources) > 0 {
			out.Meta = map[string]any{"sources": s.Sources}
		}
		return out, nil
	})

	_ = g.AddLambdaNode("build", build, saveContext)
	_ = g.AddChatModelNode("model", chatModel)
	_ = g.AddLambdaNode("reply", toReply, fillIntent)

	_ = g.AddEdge(compose.START, "build")
	_ = g.AddEdge("build", "model")
	_ = g.AddEdge("model", "reply")
	_ = g.AddEdge("reply", compose.END)
	return g, nil
}

func formatDocs(docs []*schema.Document) string {
	if len(docs) == 0 {
		return "无相关资料"
	}
	var b strings.Builder
	for i, d := range docs {
		ref := ReferenceFromDocument(d)
		fmt.Fprintf(&b, "[%d] 出处: %s\n%s\n", i+1, formatReference(ref), d.Content)
	}
	return b.String()
}

func formatReference(ref Reference) string {
	parts := []string{}
	if ref.SourceFile != "" {
		parts = append(parts, ref.SourceFile)
	}
	if ref.SourceURI != "" {
		parts = append(parts, ref.SourceURI)
	}
	if ref.PageNo > 0 {
		parts = append(parts, fmt.Sprintf("第 %d 页", ref.PageNo))
	}
	if ref.ChunkIndex > 0 {
		parts = append(parts, fmt.Sprintf("片段 %d", ref.ChunkIndex))
	}
	if ref.Scope != "" {
		parts = append(parts, string(ref.Scope))
	}
	if len(parts) == 0 {
		return ref.ID
	}
	return strings.Join(parts, "，")
}
