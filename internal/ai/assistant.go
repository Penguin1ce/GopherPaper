// Package ai 编排论文问答、抽取与报告链路,只做入口路由,不写具体链路逻辑。
//
//	问答 Chat: intent 模型分类为 chitchat/summary/method,chat 模型按子类直答或做 RAG。
//	抽取 Extract、报告 GenerateReport: 显式接口直接调对应链路。
//
// 模型与用户身份均由各链路按 ctx 的 tenant 自取(模型按用户单例,缓存在 aimodel),
// 入口不传模型句柄。与用户无关的全局检索器由 Init 一次性准备。
package ai

import (
	"context"
	"fmt"

	trpcreranker "trpc.group/trpc-go/trpc-agent-go/knowledge/reranker"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	"GopherPaper/internal/ai/agentrt"
	chatflow "GopherPaper/internal/ai/chat"
	"GopherPaper/internal/ai/core"
	extractflow "GopherPaper/internal/ai/extract"
	figureflow "GopherPaper/internal/ai/figure"
	gopherflow "GopherPaper/internal/ai/gopher"
	maodieflow "GopherPaper/internal/ai/maodie"
	pioneerflow "GopherPaper/internal/ai/pioneer"
	ragagentflow "GopherPaper/internal/ai/ragagent"
	"GopherPaper/internal/ai/retrieval"
	sessiontitleflow "GopherPaper/internal/ai/sessiontitle"
	translateflow "GopherPaper/internal/ai/translate"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/config"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
)

var ready bool

// Init 做与用户无关的一次性准备:构建全局检索器并注入 rerank 精排器(nil 时退化纯向量召回),
// 同时用 Redis 配置建小云雀的会话工作记忆存储(挂会话摘要器,摘要模型用 chatCfg)。
func Init(ctx context.Context, rr trpcreranker.Reranker, redisCfg config.RedisConfig, chatCfg config.ModelConfig) error {
	if err := retrieval.Init(ctx, rr); err != nil {
		return fmt.Errorf("init rag retriever: %w", err)
	}
	if err := pioneerflow.Init(redisCfg, chatCfg); err != nil {
		return fmt.Errorf("init pioneer session: %w", err)
	}
	ready = true
	return nil
}

// Chat 是问答入口。RAG 按当前 query 检索作答,hist 为本会话多轮上下文(来自 trpc Session)。
// ctx 须已注入租户信息供检索隔离与模型选取。
func Chat(ctx context.Context, hist []model.Message, query string) (*core.Reply, error) {
	if !ready {
		return nil, fmt.Errorf("ai: 编排器未初始化")
	}
	return chatflow.Chat(ctx, toHistory(hist), query)
}

// RewriteSessionTitle 用 intent 小模型把会话首问改写为短展示标题。
func RewriteSessionTitle(ctx context.Context, firstQuestion string) (string, error) {
	return sessiontitleflow.Rewrite(ctx, firstQuestion)
}

// FallbackSessionTitle 在小模型不可用时从首问中截取一个可用展示标题。
func FallbackSessionTitle(firstQuestion string) string {
	return sessiontitleflow.Fallback(firstQuestion)
}

// EvictUser 释放该用户常驻的 agent runner 与模型缓存,登出时调用。
// 各缓存按 userID 懒建,清除后下次访问自动重建;不影响 Redis 工作记忆与 MySQL 历史。
func EvictUser(userID string) {
	if userID == "" {
		return
	}
	pioneerflow.EvictUser(userID)
	agentrt.EvictUser(userID)
	ragagentflow.EvictUser(userID)
	gopherflow.EvictUser(userID)
	aimodel.EvictUser(userID)
}

// PioneerChat 是小云雀会话入口:不经意图分类与 RAG,直接走带工具的小云雀 agent。
// ctx 须已注入租户身份;凭据型工具的 token 由请求 ctx 携带,见 credential 包。
// 多轮上下文由 pioneer 的 Redis session 按 sessionID 自动承载(含工具轨迹),不再手工注入历史。
func PioneerChat(ctx context.Context, sessionID, query string) (*core.Reply, error) {
	// 注入思路图收集器:本轮若调了 generate_paper_flow,工具把成图写入,这里取出塞 reply.Meta 持久化。
	sink := &core.FlowSink{}
	ctx = core.WithFlowSink(ctx, sink)
	content, err := pioneerflow.Chat(ctx, sessionID, query)
	if err != nil {
		return nil, err
	}
	reply := &core.Reply{Intent: constant.IntentPioneer, Content: content}
	if flow := sink.Get(); flow != nil {
		reply.Meta = map[string]any{"flow": flow}
	}
	return reply, nil
}

// MaodieChat 是精读页小耄耋入口:围绕当前页/选段做局部问答,不走全量 agentic RAG。
func MaodieChat(ctx context.Context, hist []model.Message, query string, rc core.ReaderContext) (*core.Reply, error) {
	if !ready {
		return nil, fmt.Errorf("ai: 编排器未初始化")
	}
	return maodieflow.Chat(ctx, toHistory(hist), query, rc)
}

// toHistory 把存储层的历史消息转成 trpc 对话消息,喂给 RAG 链路做多轮上下文。
func toHistory(hist []model.Message) []trpcmodel.Message {
	if len(hist) == 0 {
		return nil
	}
	out := make([]trpcmodel.Message, 0, len(hist))
	for _, m := range hist {
		out = append(out, trpcmodel.Message{Role: trpcmodel.Role(m.Role), Content: m.Content})
	}
	return out
}

// Extract 把解析后的论文抽成结构化信息,由 parse worker 调用。ctx 须注入论文 owner。
// 模型由 agentrt 按 owner 取,故只需 ctx 带身份。
func Extract(ctx context.Context, doc *core.ParsedDoc) (*core.PaperStructured, error) {
	return extractflow.Extract(ctx, doc)
}

// DescribeFigures 给解析出的图片逐张调 vlm 生成内容描述,原地填 Figure.Desc。
// 由 parse worker 在落盘后、建图块前调用,ctx 须注入论文 owner 以选到该用户 vlm 模型。
func DescribeFigures(ctx context.Context, figs []core.Figure) error {
	return figureflow.Describe(ctx, figs)
}

// Translate 把精读页选中的英文原文译成中文,由前端显式触发,不经分类器、不走 RAG。
// 模型按用户取该用户的 translate 小模型,ctx 须注入身份。
func Translate(ctx context.Context, text string) (string, error) {
	return translateflow.Translate(ctx, text)
}

// GenerateReport 围绕某篇论文按类型生成研读报告,由前端按钮触发,不经分类器。
// 经小囊鼠的多 agent 流水线(规划→撰写→评审)生成,取代旧的一次性 report 链路。
// ctx 须注入论文 owner 供检索隔离与模型选取。
func GenerateReport(ctx context.Context, paperID string, t constant.ReportType) (*core.Reply, error) {
	return gopherflow.Generate(ctx, &core.ReportInput{
		PaperID:    paperID,
		ReportType: t,
	})
}

// GenerateRelatedResearch 沿当前论文参考文献链路收集相关研究链接。
func GenerateRelatedResearch(ctx context.Context, paperTitle string, refs []string) (*core.Reply, error) {
	return pioneerflow.RelatedResearch(ctx, paperTitle, refs)
}

// GeneratePaperFlow 围绕某篇论文生成小云雀同款节点思路图,由前端按钮触发。
// ctx 须注入论文 owner 供检索隔离与模型选取。
func GeneratePaperFlow(ctx context.Context, paperID string) (*core.Reply, error) {
	return gopherflow.GenerateFlow(ctx, paperID)
}
