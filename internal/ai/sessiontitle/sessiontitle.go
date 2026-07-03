// Package sessiontitle 用小模型把会话首问改写成适合展示的短标题。
package sessiontitle

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
)

// Rewrite 根据用户首问生成侧边栏展示标题。
func Rewrite(ctx context.Context, question string) (string, error) {
	models, err := aimodel.ModelsForUser(tenant.MustStudentID(ctx))
	if err != nil {
		return "", err
	}
	req := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(constant.SessionTitlePrompt),
			trpcmodel.NewUserMessage(strings.TrimSpace(question)),
		},
	}
	if models.IntentMC.MaxTokens > 0 {
		req.MaxTokens = &models.IntentMC.MaxTokens
	}
	out, err := core.GenerateText(ctx, models.Intent, req)
	if err != nil {
		return "", err
	}
	title := Clean(out)
	if title == "" {
		return "", fmt.Errorf("sessiontitle: 模型返回空标题")
	}
	return title, nil
}

// Fallback 从首问中截取一个可用标题,供模型失败时降级。
func Fallback(question string) string {
	title := Clean(question)
	if title == "" {
		return "论文概要"
	}
	return title
}

// Clean 清理模型输出里可能混入的引号、前缀、JSON 与多余标点。
func Clean(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if title := titleFromJSON(s); title != "" {
		s = title
	}
	s = firstNonEmptyLine(s)
	s = strings.TrimSpace(strings.Trim(s, "`"))
	for _, prefix := range []string{
		"会话标题", "短标题", "标题", "Title", "title",
	} {
		s = strings.TrimSpace(strings.TrimPrefix(s, prefix))
		s = strings.TrimSpace(strings.TrimPrefix(s, ":"))
		s = strings.TrimSpace(strings.TrimPrefix(s, "："))
	}
	s = strings.NewReplacer(
		"这篇论文", "", "这篇文章", "", "本文", "", "这个工作", "", "这项工作", "",
		"该论文", "", "该文", "",
	).Replace(s)
	s = strings.Trim(s, " \t\r\n\"'「」『』《》.,，。:：;；!！?？")
	s = trimPromptFillers(s)
	s = trimLeadingFillers(s)
	s = strings.TrimSuffix(s, "一下")
	s = collapseSpace(s)
	s = strings.Trim(s, " \t\r\n\"'「」『』《》.,，。:：;；!！?？")
	return truncateRunes(s, constant.SessionDisplayTitleMaxRunes)
}

func titleFromJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return ""
	}
	var out struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal([]byte(s[start:end+1]), &out); err != nil {
		return ""
	}
	return strings.TrimSpace(out.Title)
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func trimPromptFillers(s string) string {
	s = strings.TrimSpace(s)
	for {
		old := s
		for _, prefix := range []string{"帮我", "帮忙", "请问", "请", "能否", "能不能", "可以", "给我"} {
			s = strings.TrimSpace(strings.TrimPrefix(s, prefix))
		}
		if s == old {
			return s
		}
	}
}

func trimLeadingFillers(s string) string {
	s = strings.TrimSpace(s)
	for {
		old := s
		for _, prefix := range []string{"的", "关于", "有关"} {
			s = strings.TrimSpace(strings.TrimPrefix(s, prefix))
		}
		if s == old {
			return s
		}
	}
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if max <= 0 || len(r) <= max {
		return s
	}
	return string(r[:max])
}
