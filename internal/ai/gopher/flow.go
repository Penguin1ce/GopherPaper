// gopher flow.go 是小囊鼠的「论文思路图」子能力:复用小云雀的 JSON DAG 思路图,
// 返回节点、边与论文真实配图,由前端 PaperFlowCard 渲染。
package gopher

import (
	"context"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/toolkit"
)

// GenerateFlow 围绕某篇论文生成小云雀同款思路图 JSON。
func GenerateFlow(ctx context.Context, paperID string) (*core.Reply, error) {
	flow, err := toolkit.BuildPaperFlowGraph(ctx, paperID)
	if err != nil {
		return nil, err
	}
	return &core.Reply{
		Content: "",
		Meta: map[string]any{
			"format": "paper_flow",
			"flow":   flow,
		},
	}, nil
}
