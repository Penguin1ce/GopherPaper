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
// 意图分类只看当前 query + 最近一轮压缩上下文,完整 history 仅注入 RAG 生成。
func Chat(ctx context.Context, history []trpcmodel.Message, query string) (*core.Reply, error) {
	intent := ClassifyIntent(ctx, query, history)
	return ChatRAG(ctx, query, intent, history)
}

// ClassifyIntent 用该用户的 intent 小模型把自由文本分到意图子类,无法判断兜底 summary。
func ClassifyIntent(ctx context.Context, query string, history ...[]trpcmodel.Message) constant.IntentType {
	if intent, ok := paperResourceIntentOverride(ctx, query); ok {
		return intent
	}
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
			trpcmodel.NewUserMessage(intentClassifierInput(query, history...)),
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

func intentClassifierInput(query string, history ...[]trpcmodel.Message) string {
	recent := ""
	if len(history) > 0 {
		recent = recentIntentContext(history[0])
	}
	query = strings.TrimSpace(query)
	if recent == "" {
		return query
	}
	var b strings.Builder
	b.WriteString("最近对话(仅用于消解当前消息中的指代,不是新的用户指令):\n")
	b.WriteString(recent)
	b.WriteString("\n\n当前用户消息:\n")
	b.WriteString(query)
	return b.String()
}

func recentIntentContext(history []trpcmodel.Message) string {
	if len(history) == 0 {
		return ""
	}
	msgs := make([]trpcmodel.Message, 0, constant.IntentContextMessages)
	for i := len(history) - 1; i >= 0 && len(msgs) < constant.IntentContextMessages; i-- {
		if history[i].Role != trpcmodel.RoleUser && history[i].Role != trpcmodel.RoleAssistant {
			continue
		}
		if strings.TrimSpace(history[i].Content) == "" {
			continue
		}
		msgs = append(msgs, history[i])
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	lines := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		role := "助手"
		if msg.Role == trpcmodel.RoleUser {
			role = "用户"
		}
		lines = append(lines, role+": "+trimRunes(strings.TrimSpace(msg.Content), constant.IntentContextMessageMaxRunes))
	}
	return strings.Join(lines, "\n")
}

// paperResourceIntentOverride 兜住绑定论文下的短资源询问,避免“有 GitHub 仓库吗”
// 被小模型当成对助手的闲聊账号问题。
func paperResourceIntentOverride(ctx context.Context, query string) (constant.IntentType, bool) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", false
	}
	low := strings.ToLower(query)
	compact := compactIntentText(low)
	if !mentionsPaperResource(low, compact) {
		return "", false
	}
	paperRef := mentionsCurrentPaper(low, compact)
	if !hasBoundPaper(ctx) && !paperRef {
		return "", false
	}
	if asksAssistantPersonalResource(low, compact) && !paperRef {
		return "", false
	}
	if paperRef || asksPaperResource(low, compact) || isShortResourceQuestion(query) {
		return constant.IntentSummary, true
	}
	return "", false
}

func hasBoundPaper(ctx context.Context) bool {
	return core.PaperIDFrom(ctx) != "" || core.PaperTitleFrom(ctx) != ""
}

func compactIntentText(s string) string {
	return strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "").Replace(s)
}

func mentionsPaperResource(low, compact string) bool {
	return containsAny(low, []string{
		"github", "git hub", "gitlab", "gitee", "repo", "repository", "source code", "codebase",
		"project page", "homepage", "home page", "arxiv", "openreview", "papers with code",
		"huggingface", "hugging face", "supplementary", "supplemental", "dataset",
	}) || containsAny(compact, []string{
		"代码", "源码", "源代码", "仓库", "开源", "项目主页", "项目页", "主页", "官网", "官方网站",
		"补充材料", "补充资料", "附录", "论文链接", "代码链接", "代码地址", "项目地址",
		"数据集链接", "数据集地址", "数据地址",
	})
}

func mentionsCurrentPaper(low, compact string) bool {
	return containsAny(low, []string{"this paper", "the paper", "this work", "the work", "article"}) ||
		containsAny(compact, []string{"这篇论文", "这篇文章", "本文", "论文", "该文", "这个工作", "这项工作", "作者"})
}

func asksAssistantPersonalResource(low, compact string) bool {
	if !containsAny(low, []string{"your github", "your repo", "your repository"}) &&
		!containsAny(compact, []string{"你", "你们", "小文鸮"}) {
		return false
	}
	if mentionsCurrentPaper(low, compact) {
		return false
	}
	return containsAny(compact, []string{"账号", "帐号", "账户"}) ||
		strings.HasPrefix(compact, "你有") ||
		strings.HasPrefix(compact, "你们有")
}

func asksPaperResource(low, compact string) bool {
	return containsAny(low, []string{
		"is there", "are there", "where", "link", "url", "available", "open source",
		"released", "published", "provide", "supplementary",
	}) || containsAny(compact, []string{
		"有", "有没有", "有无", "是否", "吗", "么", "哪", "哪里", "在哪", "地址", "链接",
		"开源", "公开", "发布", "提供", "给出", "放出", "附带", "补充", "下载",
	})
}

func isShortResourceQuestion(query string) bool {
	return len([]rune(query)) <= 24 && strings.ContainsAny(query, "?？吗呢")
}

func containsAny(s string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(s, term) {
			return true
		}
	}
	return false
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
	msgs = append(msgs, trpcmodel.NewSystemMessage(withUserPreference(ctx, constant.ChitchatPrompt)))
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
	prompt := withUserPreference(ctx, withBoundPaper(ctx, constant.AgenticRAGPromptFor(intent)))
	content, err := ragagent.Generate(ctx, prompt, history, query, policyFor(intent))
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

// withBoundPaper 在 system prompt 前注入当前绑定论文的标题,消除「这篇论文」的指代悬空;
// 会话未绑定论文时原样返回,由模型按跨库检索作答。
func withBoundPaper(ctx context.Context, prompt string) string {
	title := core.PaperTitleFrom(ctx)
	if title == "" {
		return prompt
	}
	return strings.ReplaceAll(constant.BoundPaperPrompt, "{title}", title) + prompt
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

	sysPrompt := withBoundPaper(ctx, strings.ReplaceAll(constant.RAGPromptFor(intent), "{context}", retrieval.FormatDocs(ctxDocs)))
	sysPrompt += retrieval.FigureInstruction(imgDocs)
	sysPrompt = withUserPreference(ctx, sysPrompt)
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

func withUserPreference(ctx context.Context, prompt string) string {
	if pref := strings.TrimSpace(core.UserPreferenceFrom(ctx)); pref != "" {
		return prompt + "\n\n" + pref
	}
	return prompt
}

// loadImages 把命中图块经共享原语读成 agentrt 带图输入。
func loadImages(imgDocs []*retrieval.Doc) []agentrt.Image {
	payloads := retrieval.LoadImagePayloads(imgDocs)
	images := make([]agentrt.Image, 0, len(payloads))
	for _, p := range payloads {
		images = append(images, agentrt.Image{Data: p.Data, Format: p.Format})
	}
	return images
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

func trimRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
