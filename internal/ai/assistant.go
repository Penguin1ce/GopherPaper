// Package ai 编排论文问答、抽取与报告链路,只做入口路由,不写具体链路逻辑。
//
//	问答 Chat: intent 模型分类问答子类(fact/summary/method),chat 模型按子类做 RAG。
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

	chatflow "GopherPaper/internal/ai/chat"
	"GopherPaper/internal/ai/core"
	extractflow "GopherPaper/internal/ai/extract"
	figureflow "GopherPaper/internal/ai/figure"
	pioneerflow "GopherPaper/internal/ai/pioneer"
	reportflow "GopherPaper/internal/ai/report"
	"GopherPaper/internal/ai/retrieval"
	translateflow "GopherPaper/internal/ai/translate"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
)

var ready bool

// Init 只做与用户无关的一次性准备:构建全局检索器,并注入 rerank 精排器(nil 时退化纯向量召回)。
func Init(ctx context.Context, rr trpcreranker.Reranker) error {
	if err := retrieval.Init(ctx, rr); err != nil {
		return fmt.Errorf("init rag retriever: %w", err)
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

// PioneerChat 是小云雀会话入口:不经意图分类与 RAG,直接走带工具的小云雀 agent。
// ctx 须已注入租户身份;凭据型工具的 token 由请求 ctx 携带,见 credential 包。
// 多轮上下文由 pioneer 的 inmemory session 按 sessionID 自动承载(含工具轨迹),不再手工注入历史。
func PioneerChat(ctx context.Context, sessionID, query string) (*core.Reply, error) {
	content, err := pioneerflow.Chat(ctx, sessionID, query)
	if err != nil {
		return nil, err
	}
	return &core.Reply{Intent: constant.IntentPioneer, Content: content}, nil
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
// ctx 须注入论文 owner 供检索隔离与模型选取。
func GenerateReport(ctx context.Context, paperID string, t constant.ReportType) (*core.Reply, error) {
	return reportflow.Generate(ctx, &core.ReportInput{
		PaperID:    paperID,
		ReportType: t,
	})
}
