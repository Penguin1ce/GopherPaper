package paper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

// PaperFlow 围绕某篇论文生成小云雀同款节点思路图,仅限本人。
// 校验归属与就绪态后优先读持久化缓存,未命中再同步生成并落库。
// ctx 注入 owner 供检索隔离与模型选取。
func PaperFlow(ctx context.Context, ownerID, paperID string) (*core.Reply, error) {
	p, err := owned(ctx, ownerID, paperID)
	if err != nil {
		return nil, err
	}
	// 思路图依赖向量库里的论文分块,未入库则无证据可检,提前给友好提示。
	if p.Status != constant.PaperIndexed && p.Status != constant.PaperReady {
		return nil, errs.ErrPaperNotReady
	}
	if reply, err := GetPaperFlow(ctx, ownerID, paperID); err == nil {
		return reply, nil
	} else if !errors.Is(err, errs.ErrPaperFlowNotFound) {
		return nil, err
	}
	// ctx 由 HTTP 中间件注入了 tenant 身份(owner 即取自其中),下游检索与模型按此隔离,无需再注入。
	reply, err := ai.GeneratePaperFlow(ctx, paperID)
	if err != nil {
		return nil, err
	}
	if reply.Meta == nil {
		return nil, fmt.Errorf("service/paper: 思路图生成结果缺少 meta")
	}
	flow, ok := reply.Meta["flow"]
	if !ok {
		return nil, fmt.Errorf("service/paper: 思路图生成结果缺少 flow")
	}
	flowJSON, err := normalizeFlowJSON(flow)
	if err != nil {
		return nil, err
	}
	if err := paperdao.SavePaperFlow(ctx, &model.PaperFlowCache{
		PaperID:  paperID,
		OwnerID:  ownerID,
		FlowJSON: model.JSONMap(flowJSON),
	}); err != nil {
		return nil, err
	}
	return reply, nil
}

// GetPaperFlow 只读取已生成的思路图缓存,不触发昂贵生成。
func GetPaperFlow(ctx context.Context, ownerID, paperID string) (*core.Reply, error) {
	if _, err := owned(ctx, ownerID, paperID); err != nil {
		return nil, err
	}
	cached, err := paperdao.GetPaperFlow(ctx, ownerID, paperID)
	if err != nil {
		return nil, err
	}
	return &core.Reply{Meta: map[string]any{
		"format": "paper_flow",
		"flow":   map[string]any(cached.FlowJSON),
		"cached": true,
	}}, nil
}

func normalizeFlowJSON(flow any) (map[string]any, error) {
	blob, err := json.Marshal(flow)
	if err != nil {
		return nil, fmt.Errorf("service/paper: 序列化思路图失败: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(blob, &out); err != nil {
		return nil, fmt.Errorf("service/paper: 解析思路图失败: %w", err)
	}
	return out, nil
}
