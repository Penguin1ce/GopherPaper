// extract_trpc.go 是 extract 链路:把解析后的论文正文抽成结构化信息。
// 复用 bodyText/parseStructured 做容错 JSON 解析,生成参数走调用时的 Request.GenerationConfig。
package extract_pipeline

import (
	"context"
	"strings"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	trpcopenai "trpc.group/trpc-go/trpc-agent-go/model/openai"

	"GopherPaper/internal/agent"
	"GopherPaper/internal/config"
	"GopherPaper/pkg/constant"
)

// ExtractTRPC 用 trpc model 把解析后的论文抽成结构化信息。
func ExtractTRPC(ctx context.Context, m *trpcopenai.Model, mc config.ModelConfig, doc *agent.ParsedDoc) (*agent.PaperStructured, error) {
	sysPrompt := strings.ReplaceAll(constant.ExtractPrompt, "{context}", bodyText(doc))
	req := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(sysPrompt),
			// 须带 user 轮次,否则推理模型只见 system 指令会返回空 content。
			trpcmodel.NewUserMessage("请基于以上论文正文输出抽取的 JSON。"),
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
	return parseStructured(content)
}
