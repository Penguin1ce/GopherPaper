// extract_trpc.go 是 extract 链路:把解析后的论文正文抽成结构化信息。
// 复用 bodyText/parseStructured 做容错 JSON 解析,生成参数走调用时的 Request.GenerationConfig。
package extract

import (
	"context"
	"strings"

	"GopherPaper/internal/ai/agentrt"
	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
)

// ExtractTRPC 经带工具 chat agent 把解析后的论文抽成结构化信息。ctx 须注入论文 owner。
func ExtractTRPC(ctx context.Context, doc *core.ParsedDoc) (*core.PaperStructured, error) {
	sysPrompt := strings.ReplaceAll(constant.ExtractPrompt, "{context}", bodyText(doc))
	// 须带 user 轮次,否则推理模型只见 system 指令会返回空 content。
	content, err := agentrt.Generate(ctx, tenant.MustStudentID(ctx), sysPrompt, nil, "请基于以上论文正文输出抽取的 JSON。")
	if err != nil {
		return nil, err
	}
	return parseStructured(content)
}
