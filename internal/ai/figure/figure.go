// Package figure 是解析期的图描述链路:给论文插图/表格逐张调 vlm 生成内容描述。
// 描述与 caption 一起入库,让"按图内容"也能召回。裸调 vlm 不挂工具,与问答链路解耦。
package figure

import (
	"context"
	"path/filepath"
	"strings"
	"sync"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	trpcopenai "trpc.group/trpc-go/trpc-agent-go/model/openai"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/config"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// Describe 给有图片字节的 Figure 逐张调该用户的 vlm 生成内容描述,原地填 Desc。
// 一张一张发(每图一段独立描述对应入库),并发受限;每篇最多描述 MaxFigureDescribe 张,
// 单图失败只记日志不填 Desc,不阻断整篇入库。ctx 须注入论文 owner。
func Describe(ctx context.Context, figs []core.Figure) error {
	models, err := aimodel.ModelsForUser(ctx, tenant.MustStudentID(ctx))
	if err != nil {
		return err
	}
	vlm, mc := models.Vlm, models.VlmMC
	if vlm == nil {
		return nil
	}
	sem := make(chan struct{}, constant.FigureDescribeWorker)
	var wg sync.WaitGroup
	described := 0
	for i := range figs {
		if len(figs[i].ImgData) == 0 {
			continue
		}
		if described >= constant.MaxFigureDescribe {
			break
		}
		described++
		wg.Add(1)
		sem <- struct{}{}
		go func(fig *core.Figure) {
			defer wg.Done()
			defer func() { <-sem }()
			desc, err := describeOne(ctx, vlm, mc, fig)
			if err != nil {
				zlog.Error("图片描述生成失败,跳过", "img", fig.ImgPath, "err", err)
				return
			}
			fig.Desc = strings.TrimSpace(desc)
		}(&figs[i])
	}
	wg.Wait()
	return nil
}

// describeOne 给单张图构造带图 user 轮次裸调 vlm,system 带描述指令。
func describeOne(ctx context.Context, vlm *trpcopenai.Model, mc config.ModelConfig, fig *core.Figure) (string, error) {
	hint := "请描述这张论文图片。"
	user := trpcmodel.Message{Role: trpcmodel.RoleUser}
	user.ContentParts = append(user.ContentParts, trpcmodel.ContentPart{Type: trpcmodel.ContentTypeText, Text: &hint})
	user.AddImageData(fig.ImgData, "auto", imageFormat(fig.ImgPath))
	req := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(constant.FigureDescribePrompt),
			user,
		},
	}
	if mc.MaxTokens > 0 {
		req.MaxTokens = &mc.MaxTokens
	}
	return core.GenerateText(ctx, vlm, req)
}

// imageFormat 从图片路径扩展名推出 vlm 需要的 format(不带点),无法识别回退 png。
func imageFormat(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "jpg", "jpeg", "png", "webp", "gif":
		return ext
	default:
		return "png"
	}
}
