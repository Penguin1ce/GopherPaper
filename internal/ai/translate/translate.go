// Package translate 是精读页的逐段翻译链路:把用户选中的英文学术原文译成中文。
// 裸调小模型不挂工具、不走 RAG,与问答/抽取/报告解耦,可单独换型。
package translate

import (
	"context"
	"fmt"
	"strings"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
)

// Translate 把英文原文 text 翻成中文。system 带翻译指令,user 给原文,裸调该用户的
// translate 小模型聚合成文本。text 为空或超长由上层先行校验,这里只做最终保护。
func Translate(ctx context.Context, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("translate: 原文为空")
	}
	models, err := aimodel.ModelsForUser(tenant.MustStudentID(ctx))
	if err != nil {
		return "", err
	}
	m, mc := models.Translate, models.TranslateMC
	if m == nil {
		return "", fmt.Errorf("translate: 模型未配置")
	}
	req := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(constant.TranslatePrompt),
			trpcmodel.NewUserMessage(text),
		},
	}
	if mc.MaxTokens > 0 {
		req.MaxTokens = &mc.MaxTokens
	}
	return core.GenerateText(ctx, m, req)
}
