// chat.go 是 chat 链路:intent 模型分类 + chat 模型参数化 RAG,显式两段编排。
//
// 三个问答子类(fact/summary/method)本就共用同一参数化 RAG agent(仅 prompt 不同),
// 故不做自主路由,而是显式两段:
//
//	1 ClassifyIntent: intent 小模型把自由文本分到问答子类
//	2 ChatRAG:        chat 模型按子类选 prompt 做 RAG,并调用 retriever
//
// 二者解耦、两模型分用,忠实 CLAUDE.md「小模型意图识别 + 下游 RAG」的设计。
// 模型由各段按 ctx 的 tenant 自取,调用方不传模型与身份。
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	"GopherPaper/internal/ai/agentrt"
	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// Chat 是 chat 切片入口:intent 小模型分类 + 带工具 chat agent 做 RAG。
// history 为本会话多轮上下文(来自 trpc Session),按时间升序、不含当前 query;
// 意图分类只看当前 query,history 仅注入 RAG 生成。
func Chat(ctx context.Context, history []trpcmodel.Message, query string) (*core.Reply, error) {
	intent := ClassifyIntent(ctx, query)
	return ChatRAG(ctx, query, intent, history)
}

// ClassifyIntent 用该用户的 intent 小模型把自由文本分到问答子类,无法判断兜底 summary。
func ClassifyIntent(ctx context.Context, query string) constant.IntentType {
	models, err := aimodel.ModelsForUser(tenant.MustStudentID(ctx))
	if err != nil {
		zlog.Error("意图分类取模型失败,兜底 summary", "err", err)
		return constant.IntentSummary
	}
	// IntentPrompt 的花括号是双写转义的模板写法,此处不走模板,还原成普通 JSON 示例。
	prompt := strings.NewReplacer("{{", "{", "}}", "}").Replace(constant.IntentPrompt)
	req := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(prompt),
			trpcmodel.NewUserMessage(query),
		},
	}
	if models.IntentMC.MaxTokens > 0 {
		req.MaxTokens = &models.IntentMC.MaxTokens
	}
	content, err := core.GenerateText(ctx, models.Intent, req)
	if err != nil {
		zlog.Error("意图分类失败,兜底 summary", "err", err)
		return constant.IntentSummary
	}
	return parseIntent(content)
}

// ChatRAG 按意图做参数化 RAG:检索片段拼进 system prompt,经带工具 chat agent 生成并收集出处进 Meta。
// history 为多轮上下文,经 agent 注入,夹在 system prompt 与当前 query 之间。
// 图块走单独一轮检索(不与正文同池),命中的图片随 query 一起发给多模态 chat 模型推理。
func ChatRAG(ctx context.Context, query string, intent constant.IntentType, history []trpcmodel.Message) (*core.Reply, error) {
	owner := tenant.MustStudentID(ctx)
	paperID := core.PaperIDFrom(ctx)
	docs, err := RetrieveForPaper(ctx, query, owner, paperID)
	if err != nil {
		zlog.Error("RAG 检索失败", "owner", owner, "paper_id", paperID, "err", err)
		docs = nil
	}
	docs = dropImageDocs(docs) // 正文上下文不含图块,图块由下方单独一轮专管
	imgDocs, err := RetrieveImagesForPaper(ctx, query, owner, paperID)
	if err != nil {
		zlog.Error("图块检索失败", "owner", owner, "paper_id", paperID, "err", err)
		imgDocs = nil
	}
	if len(docs) == 0 && len(imgDocs) == 0 {
		zlog.Info("RAG 未召回到任何片段", "owner", owner, "paper_id", paperID)
	}

	// 正文片段与命中图的说明一起拼进 context,图片本体单独 base64 发给模型;出处含正文与图。
	ctxDocs := append(append([]*Doc{}, docs...), imgDocs...)
	sources := References(ctxDocs)
	images := loadImages(imgDocs)

	sysPrompt := strings.ReplaceAll(constant.RAGPromptFor(intent), "{context}", FormatDocs(ctxDocs))
	sysPrompt += figureInstruction(imgDocs)
	content, err := agentrt.GenerateWithImages(ctx, sysPrompt, history, query, images)
	if err != nil {
		return nil, err
	}
	reply := &core.Reply{Content: content, Intent: intent}
	if len(sources) > 0 {
		reply.Meta = map[string]any{"sources": sources}
	}
	return reply, nil
}

// figureInstruction 在有召回图时追加插图指示:让模型用 figure://文件名 占位把图插进正文对应位置,
// 文件名只能取自下方清单(即图块图片名),前端再把占位解析成带 token 的取图 URL。
func figureInstruction(imgDocs []*Doc) string {
	if len(imgDocs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n下面是与问题相关、已随消息提供给你的图片。若某张图能直观支撑回答,请用 Markdown 图片语法 ![简短说明](figure://文件名) 把它插入到正文对应位置;文件名只能用下面列出的,不要编造,不需要时不必插图:\n")
	for _, d := range imgDocs {
		name := filepath.Base(metaString(d, constant.MilvusFieldImgURI))
		if name == "" {
			continue
		}
		fmt.Fprintf(&b, "- figure://%s : %s\n", name, summarize(d.Content, 40))
	}
	return b.String()
}

// summarize 把图块说明压成单行短摘要,作插图清单的图片标注。
func summarize(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// loadImages 把命中图块的本地图片读成带图问答的 Image,单张读失败只记日志跳过(其 caption 仍在 context)。
func loadImages(imgDocs []*Doc) []agentrt.Image {
	images := make([]agentrt.Image, 0, len(imgDocs))
	for _, d := range imgDocs {
		uri := metaString(d, constant.MilvusFieldImgURI)
		if uri == "" {
			continue
		}
		data, err := os.ReadFile(uri)
		if err != nil {
			zlog.Error("读取召回图片失败,跳过", "img_uri", uri, "err", err)
			continue
		}
		images = append(images, agentrt.Image{Data: data, Format: imageFormat(uri)})
	}
	return images
}

// imageFormat 从图片路径扩展名推出模型需要的 format(不带点),无法识别回退 png。
func imageFormat(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "jpg", "jpeg", "png", "webp", "gif":
		return ext
	default:
		return "png"
	}
}

// parseIntent 容错解析意图分类输出,非法兜底 summary。
func parseIntent(content string) constant.IntentType {
	var out struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(extractJSON(content)), &out); err == nil {
		switch constant.IntentType(out.Type) {
		case constant.IntentFact:
			return constant.IntentFact
		case constant.IntentMethod:
			return constant.IntentMethod
		case constant.IntentSummary:
			return constant.IntentSummary
		}
	}
	// JSON 解析失败时退化到关键词匹配。
	low := strings.ToLower(content)
	switch {
	case strings.Contains(low, "fact"):
		return constant.IntentFact
	case strings.Contains(low, "method"):
		return constant.IntentMethod
	default:
		return constant.IntentSummary
	}
}

// extractJSON 截取首个 { 到末个 },剥离可能的代码围栏与多余文字。
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
