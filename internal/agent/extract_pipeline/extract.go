// Package extract_pipeline 实现论文结构化抽取 agent，把解析后的正文抽成 PaperStructured。
// 由 parse worker 在 PDF 解析完成后调用，输出题目/作者/方法/结果/创新点等字段。
package extract_pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"GopherPaper/internal/agent"
	"GopherPaper/pkg/constant"
)

// maxBodyChars 喂给模型的正文字符上限，超长截断避免超出上下文窗。
const maxBodyChars = 12000

// Build 构造结构化抽取 agent 子图。
func Build(chatModel model.BaseChatModel) (*compose.Graph[*agent.ParsedDoc, *agent.PaperStructured], error) {
	g := compose.NewGraph[*agent.ParsedDoc, *agent.PaperStructured]()

	prepare := compose.InvokableLambda(func(_ context.Context, doc *agent.ParsedDoc) ([]*schema.Message, error) {
		sysPrompt := strings.ReplaceAll(constant.ExtractPrompt, "{context}", bodyText(doc))
		return []*schema.Message{schema.SystemMessage(sysPrompt)}, nil
	})
	toStruct := compose.InvokableLambda(func(_ context.Context, msg *schema.Message) (*agent.PaperStructured, error) {
		return parseStructured(msg.Content)
	})

	_ = g.AddLambdaNode("prepare", prepare)
	_ = g.AddChatModelNode("model", chatModel)
	_ = g.AddLambdaNode("parse", toStruct)
	_ = g.AddEdge(compose.START, "prepare")
	_ = g.AddEdge("prepare", "model")
	_ = g.AddEdge("model", "parse")
	_ = g.AddEdge("parse", compose.END)
	return g, nil
}

// bodyText 把段落按章节拼成正文，带截断。
func bodyText(doc *agent.ParsedDoc) string {
	var b strings.Builder
	lastSection := ""
	for _, p := range doc.Paragraphs {
		if p.SectionPath != "" && p.SectionPath != lastSection {
			b.WriteString("\n## ")
			b.WriteString(p.SectionPath)
			b.WriteString("\n")
			lastSection = p.SectionPath
		}
		b.WriteString(p.Text)
		b.WriteString("\n")
		if b.Len() >= maxBodyChars {
			break
		}
	}
	out := b.String()
	if len(out) > maxBodyChars {
		out = out[:maxBodyChars]
	}
	return out
}

// parseStructured 容错解析模型返回的 JSON，剥离可能的代码围栏。
func parseStructured(content string) (*agent.PaperStructured, error) {
	raw := stripFence(content)
	var s agent.PaperStructured
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("extract_pipeline: 解析抽取结果失败: %w", err)
	}
	return &s, nil
}

// stripFence 去掉 ```json ... ``` 围栏，并截取首个 { 到末个 }。
func stripFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return strings.TrimSpace(s)
}
