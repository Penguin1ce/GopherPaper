// chat_trpc.go 是 chat 链路:intent 模型分类 + chat 模型参数化 RAG,显式两段编排。
//
// 三个问答子类(fact/summary/method)本就共用同一参数化 RAG agent(仅 prompt 不同),
// 故不做自主路由,而是显式两段:
//
//	1 ClassifyIntentTRPC: intent 小模型把自由文本分到问答子类
//	2 ChatRAGTRPC:        chat 模型按子类选 prompt 做 RAG，并调用 retriever
//
// 二者解耦、两模型分用,忠实 CLAUDE.md「小模型意图识别 + 下游 RAG」的设计。
package chat

import (
	"context"
	"encoding/json"
	"strings"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	trpcopenai "trpc.group/trpc-go/trpc-agent-go/model/openai"

	"GopherPaper/internal/ai/agentrt"
	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/config"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// ChatTRPC 是 chat 切片入口:intent 小模型分类 + 带工具 chat agent 做 RAG。
// history 为本会话多轮上下文(来自 trpc Session),按时间升序、不含当前 query;
// 意图分类只看当前 query,history 仅注入 RAG 生成。
func ChatTRPC(ctx context.Context, intentModel *trpcopenai.Model, intentMC config.ModelConfig, history []trpcmodel.Message, query string) (*core.Reply, error) {
	intent := ClassifyIntentTRPC(ctx, intentModel, intentMC, query)
	return ChatRAGTRPC(ctx, query, intent, history)
}

// ClassifyIntentTRPC 用 intent 小模型把自由文本分到问答子类,无法判断兜底 summary。
func ClassifyIntentTRPC(ctx context.Context, intentModel *trpcopenai.Model, mc config.ModelConfig, query string) constant.IntentType {
	// IntentPrompt 的花括号是双写转义的模板写法,此处不走模板,还原成普通 JSON 示例。
	prompt := strings.NewReplacer("{{", "{", "}}", "}").Replace(constant.IntentPrompt)
	req := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(prompt),
			trpcmodel.NewUserMessage(query),
		},
	}
	if mc.MaxTokens > 0 {
		req.MaxTokens = &mc.MaxTokens
	}
	content, err := core.GenerateText(ctx, intentModel, req)
	if err != nil {
		zlog.Error("意图分类失败,兜底 summary", "err", err)
		return constant.IntentSummary
	}
	return parseIntent(content)
}

// ChatRAGTRPC 按意图做参数化 RAG:检索片段拼进 system prompt,经带工具 chat agent 生成并收集出处进 Meta。
// history 为多轮上下文,经 agent 注入,夹在 system prompt 与当前 query 之间。
func ChatRAGTRPC(ctx context.Context, query string, intent constant.IntentType, history []trpcmodel.Message) (*core.Reply, error) {
	owner := tenant.MustStudentID(ctx)
	paperID := core.PaperIDFrom(ctx)
	docs, err := RetrieveForPaper(ctx, query, owner, paperID)
	if err != nil {
		zlog.Error("RAG 检索失败", "owner", owner, "paper_id", paperID, "err", err)
		docs = nil
	} else if len(docs) == 0 {
		zlog.Info("RAG 未召回到任何片段", "owner", owner, "paper_id", paperID)
	}
	sources := References(docs)

	sysPrompt := strings.ReplaceAll(constant.RAGPromptFor(intent), "{context}", FormatDocs(docs))
	content, err := agentrt.Generate(ctx, owner, sysPrompt, history, query)
	if err != nil {
		return nil, err
	}
	reply := &core.Reply{Content: content, Intent: intent}
	if len(sources) > 0 {
		reply.Meta = map[string]any{"sources": sources}
	}
	return reply, nil
}

// parseIntent 容错解析意图分类输出,非法兜底 summary。
func parseIntent(content string) constant.IntentType {
	var out struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(extractJSON(content)), &out); err == nil {
		switch constant.IntentType(out.Type) {
		case constant.IntentFact:
			return constant.IntentFact
		case constant.IntentMethod:
			return constant.IntentMethod
		case constant.IntentSummary:
			return constant.IntentSummary
		}
	}
	// JSON 解析失败时退化到关键词匹配。
	low := strings.ToLower(content)
	switch {
	case strings.Contains(low, "fact"):
		return constant.IntentFact
	case strings.Contains(low, "method"):
		return constant.IntentMethod
	default:
		return constant.IntentSummary
	}
}

// extractJSON 截取首个 { 到末个 },剥离可能的代码围栏与多余文字。
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
