// chat.go 是 chat 链路:intent 模型分类 + chat 模型参数化 RAG,显式两段编排。
//
// chitchat/summary/method 三类由 intent 模型选择:
// chitchat 直接对话,summary/method 走同一套 agentic RAG。
// 故不做自主路由,而是显式两段:
//
//	1 ClassifyIntent: intent 小模型把自由文本分到意图子类
//	2 ChatRAG:        chat 模型按子类直答或做 RAG
//
// 二者解耦、两模型分用,忠实 CLAUDE.md「小模型意图识别 + 下游 RAG」的设计。
// 模型由各段按 ctx 的 tenant 自取,调用方不传模型与身份。
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	"GopherPaper/internal/ai/agentrt"
	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/planstream"
	"GopherPaper/internal/ai/ragagent"
	"GopherPaper/internal/ai/retrieval"
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

// ClassifyIntent 用该用户的 intent 小模型把自由文本分到意图子类,无法判断兜底 summary。
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

// ChatRAG 按意图分流:
//   - chitchat 走直答快路径:不检索论文,chat 模型带历史直接对话(闲聊无需 RAG,省检索与延迟);
//   - summary/method(含原 fact 类事实定位)走 agentic 循环(ragagent):agent 自主规划→检索→反思→决策,
//     多轮按需检索——事实型问题常需跨片段综合,交给 agent 自定检索深度比单轮快路径更稳。
//
// history 为多轮上下文,经 agent 注入,夹在 system prompt 与当前 query 之间。RAG 路把出处收进 Reply.Meta。
func ChatRAG(ctx context.Context, query string, intent constant.IntentType, history []trpcmodel.Message) (*core.Reply, error) {
	if intent == constant.IntentChitchat {
		return chitchatReply(ctx, query, history)
	}
	return agenticRAG(ctx, query, intent, history)
}

// chitchatReply 处理闲聊:不检索论文,用 chat 模型带历史直接对话作答。
func chitchatReply(ctx context.Context, query string, history []trpcmodel.Message) (*core.Reply, error) {
	models, err := aimodel.ModelsForUser(tenant.MustStudentID(ctx))
	if err != nil {
		return nil, err
	}
	msgs := make([]trpcmodel.Message, 0, len(history)+2)
	msgs = append(msgs, trpcmodel.NewSystemMessage(constant.ChitchatPrompt))
	msgs = append(msgs, history...)
	msgs = append(msgs, trpcmodel.NewUserMessage(query))
	req := &trpcmodel.Request{Messages: msgs}
	if models.ChatMC.MaxTokens > 0 {
		req.MaxTokens = &models.ChatMC.MaxTokens
	}
	content, err := core.GenerateText(ctx, models.Chat, req)
	if err != nil {
		return nil, err
	}
	return &core.Reply{Content: content, Intent: constant.IntentChitchat}, nil
}

// agenticRAG 让 agent 在主循环里自主多轮检索作答(summary/method)。
// 出处散在各轮工具调用中,故挂 ctx 引用收集器,循环结束后排空填进 Meta。
func agenticRAG(ctx context.Context, query string, intent constant.IntentType, history []trpcmodel.Message) (*core.Reply, error) {
	ctx = retrieval.WithRefSink(ctx)
	content, err := ragagent.Generate(ctx, constant.AgenticRAGPromptFor(intent), history, query, policyFor(intent))
	if err != nil {
		if errors.Is(err, planstream.ErrPseudoToolCall) {
			zlog.Warn("agentic RAG 输出伪工具调用,降级单轮 RAG", "intent", intent, "err", err)
			return singleShotRAG(ctx, query, history, intent)
		}
		if errors.Is(err, planstream.ErrMaxToolIterations) {
			zlog.Warn("agentic RAG 工具轮数耗尽,降级单轮 RAG", "intent", intent, "err", err)
			return singleShotRAG(ctx, query, history, intent)
		}
		return nil, err
	}
	reply := &core.Reply{Content: content, Intent: intent}
	if sources := retrieval.DrainRefs(ctx); len(sources) > 0 {
		reply.Meta = map[string]any{"sources": sources}
	}
	return reply, nil
}

// policyFor 按问答子类给 agentic 循环定工具迭代预算:方法类常需逐步检索故放宽,其余按概括预算。
func policyFor(intent constant.IntentType) ragagent.Policy {
	if intent == constant.IntentMethod {
		return ragagent.Policy{MaxIter: constant.AgenticMaxIterMethod}
	}
	return ragagent.Policy{MaxIter: constant.AgenticMaxIterSummary}
}

// singleShotRAG 是单轮 RAG:预检索正文与图块各一次拼进 system prompt,命中图随 query 发给
// 多模态 chat 模型,单轮生成并收集出处进 Meta。仅作 agentic 链路输出伪工具调用时的兜底。
func singleShotRAG(ctx context.Context, query string, history []trpcmodel.Message, intent constant.IntentType) (*core.Reply, error) {
	owner := tenant.MustStudentID(ctx)
	paperID := core.PaperIDFrom(ctx)
	docs, err := retrieval.RetrieveForPaper(ctx, query, owner, paperID)
	if err != nil {
		zlog.Error("RAG 检索失败", "owner", owner, "paper_id", paperID, "err", err)
		docs = nil
	}
	docs = retrieval.DropImageDocs(docs) // 正文上下文不含图块,图块由下方单独一轮专管
	imgDocs, err := retrieval.RetrieveImagesForPaper(ctx, query, owner, paperID)
	if err != nil {
		zlog.Error("图块检索失败", "owner", owner, "paper_id", paperID, "err", err)
		imgDocs = nil
	}
	if len(docs) == 0 && len(imgDocs) == 0 {
		zlog.Info("RAG 未召回到任何片段", "owner", owner, "paper_id", paperID)
	}

	// 正文片段与命中图的说明一起拼进 context,图片本体单独 base64 发给模型;出处含正文与图。
	ctxDocs := append(append([]*retrieval.Doc{}, docs...), imgDocs...)
	sources := retrieval.References(ctxDocs)
	images := loadImages(imgDocs)

	sysPrompt := strings.ReplaceAll(constant.RAGPromptFor(intent), "{context}", retrieval.FormatDocs(ctxDocs))
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
func figureInstruction(imgDocs []*retrieval.Doc) string {
	if len(imgDocs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n下面是与问题相关、已随消息提供给你的图片。只要图能直观支撑回答,就用 Markdown 图片语法 ![简短说明](figure://文件名) 把它插入到正文对应位置,并在正文里点明该图说明了什么;文件名只能用下面列出的,不要编造,确实没有相关图时才不插:\n")
	for _, d := range imgDocs {
		name := filepath.Base(retrieval.MetaString(d, constant.MilvusFieldImgURI))
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
func loadImages(imgDocs []*retrieval.Doc) []agentrt.Image {
	images := make([]agentrt.Image, 0, len(imgDocs))
	for _, d := range imgDocs {
		uri := retrieval.MetaString(d, constant.MilvusFieldImgURI)
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
		typ := strings.ToLower(strings.TrimSpace(out.Type))
		switch constant.IntentType(typ) {
		case constant.IntentChitchat:
			return constant.IntentChitchat
		case constant.IntentMethod:
			return constant.IntentMethod
		case constant.IntentSummary:
			return constant.IntentSummary
		}
		if typ == "fact" {
			return constant.IntentSummary
		}
	}
	// JSON 解析失败时退化到关键词匹配。
	low := strings.ToLower(content)
	switch {
	case strings.Contains(low, "chitchat") || strings.Contains(content, "闲聊"):
		return constant.IntentChitchat
	case strings.Contains(low, "method"):
		return constant.IntentMethod
	case strings.Contains(low, "fact"):
		return constant.IntentSummary
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
