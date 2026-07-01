package paper

import (
	"context"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

// PaperFlow 围绕某篇论文画一张研究思路流程图(自包含 SVG),仅限本人。
// 校验归属与就绪态后同步经小囊鼠单轮 agent 生成,不缓存、不落库、不走后台队列。
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
	// ctx 由 HTTP 中间件注入了 tenant 身份(owner 即取自其中),下游检索与模型按此隔离,无需再注入。
	return ai.GeneratePaperFlowSVG(ctx, paperID)
}
