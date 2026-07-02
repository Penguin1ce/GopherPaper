package ai

import (
	"context"
	"encoding/json"
	"fmt"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
)

// ComparePapers 基于多篇论文的结构化抽取结果生成横向对比分析。
func ComparePapers(ctx context.Context, papers []core.PaperCompareInput) (*core.Reply, error) {
	if len(papers) < 2 {
		return nil, fmt.Errorf("ai: 至少需要两篇论文才能对比")
	}
	userID := tenant.MustStudentID(ctx)
	models, err := aimodel.ModelsForUser(userID)
	if err != nil {
		return nil, err
	}
	payload, err := json.MarshalIndent(papers, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("ai: 序列化论文对比输入失败: %w", err)
	}
	req := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(constant.ComparePapersPrompt),
			trpcmodel.NewUserMessage("请对比下面这些论文，生成方法对比表和差异分析说明：\n\n" + string(payload)),
		},
	}
	if models.ChatMC.MaxTokens > 0 {
		req.MaxTokens = &models.ChatMC.MaxTokens
	}
	content, err := core.GenerateText(ctx, models.Chat, req)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(papers))
	for _, p := range papers {
		ids = append(ids, p.ID)
	}
	return &core.Reply{
		Intent:  constant.IntentType("compare"),
		Content: content,
		Meta:    map[string]any{"paper_ids": ids},
	}, nil
}
