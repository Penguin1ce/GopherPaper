// Package ai 编排论文问答、抽取与报告链路。
//
//	问答 Chat: intent 模型分类问答子类(fact/summary/method),chat 模型按子类做 RAG。
//	抽取 Extract、报告 GenerateReport: 显式接口直接调对应链路。
//
// 三条链路均为 trpc-agent-go 实现(chat/extract/report 的 *_trpc.go),模型按用户单例。
// 与用户无关的全局检索器由 Init 一次性准备。
package ai

import (
	"context"
	"fmt"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	chatflow "GopherPaper/internal/ai/chat"
	"GopherPaper/internal/ai/core"
	extractflow "GopherPaper/internal/ai/extract"
	reportflow "GopherPaper/internal/ai/report"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/model"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
)

var ready bool

// Init 只做与用户无关的一次性准备:构建全局检索器。
func Init(ctx context.Context) error {
	if err := chatflow.Init(ctx); err != nil {
		return fmt.Errorf("init rag retriever: %w", err)
	}
	ready = true
	return nil
}

// modelsForUser 返回单用户模型集合,模型缓存由 aimodel 统一负责。
func modelsForUser(userID string) (*aimodel.ModelSet, error) {
	if !ready {
		return nil, fmt.Errorf("ai: 编排器未初始化")
	}
	if userID == "" {
		return nil, fmt.Errorf("ai: 缺少用户身份")
	}
	return aimodel.ModelsForUser(userID)
}

// Chat 是问答入口。RAG 按当前 query 检索作答,hist 为本会话多轮上下文(来自 trpc Session)。
// ctx 须已注入租户信息供检索隔离。
func Chat(ctx context.Context, hist []model.Message, query string) (*core.Reply, error) {
	models, err := modelsForUser(tenant.MustStudentID(ctx))
	if err != nil {
		return nil, err
	}
	return chatflow.ChatTRPC(ctx, models.Intent, models.IntentMC, toHistory(hist), query)
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
	return extractflow.ExtractTRPC(ctx, doc)
}

// GenerateReport 围绕某篇论文按类型生成研读报告,由前端按钮触发,不经分类器。
func GenerateReport(ctx context.Context, paperID string, t constant.ReportType) (*core.Reply, error) {
	return reportflow.GenerateReportTRPC(ctx, &core.ReportInput{
		PaperID:    paperID,
		OwnerID:    tenant.MustStudentID(ctx),
		ReportType: t,
	})
}
