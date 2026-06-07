// Package ai 编排论文问答的下游链路,运行器按 userID 懒建并缓存。
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
	"sync"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	localagent "GopherPaper/internal/agent"
	"GopherPaper/internal/agent/chat_pipeline"
	"GopherPaper/internal/agent/extract_pipeline"
	"GopherPaper/internal/agent/report_pipeline"
	"GopherPaper/internal/factory"
	"GopherPaper/internal/model"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
)

// userRunner 持有单个用户的 trpc 模型集合,可并发复用。
type userRunner struct {
	models *factory.TRPCUserModels
}

var (
	ready       bool
	userRunners sync.Map // userID -> *runnerEntry
)

type runnerEntry struct {
	once   sync.Once
	runner *userRunner
	err    error
}

// Init 只做与用户无关的一次性准备:构建全局检索器。模型与 runner 按用户懒建。
func Init(ctx context.Context) error {
	if err := chat_pipeline.Init(ctx); err != nil {
		return fmt.Errorf("init rag retriever: %w", err)
	}
	ready = true
	return nil
}

// runnerFor 返回某用户的运行器,未命中则取该用户模型并缓存。
func runnerFor(userID string) (*userRunner, error) {
	if !ready {
		return nil, fmt.Errorf("ai: 编排器未初始化")
	}
	if userID == "" {
		return nil, fmt.Errorf("ai: 缺少用户身份")
	}
	e, _ := userRunners.LoadOrStore(userID, &runnerEntry{})
	ent := e.(*runnerEntry)
	ent.once.Do(func() {
		ent.runner, ent.err = buildUserRunner(userID)
	})
	if ent.err != nil {
		userRunners.Delete(userID)
		return nil, ent.err
	}
	return ent.runner, nil
}

func buildUserRunner(userID string) (*userRunner, error) {
	um, err := factory.TRPCModelsForUser(userID)
	if err != nil {
		return nil, err
	}
	return &userRunner{models: um}, nil
}

// Chat 是问答入口。RAG 按当前 query 检索作答,hist 为本会话多轮上下文(来自 trpc Session)。
// ctx 须已注入租户信息供检索隔离。
func Chat(ctx context.Context, hist []model.Message, query string) (*localagent.Reply, error) {
	r, err := runnerFor(tenant.MustStudentID(ctx))
	if err != nil {
		return nil, err
	}
	return chat_pipeline.ChatTRPC(ctx, r.models.Intent, r.models.Chat, r.models.IntentMC, r.models.ChatMC, toHistory(hist), query)
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
func Extract(ctx context.Context, doc *localagent.ParsedDoc) (*localagent.PaperStructured, error) {
	r, err := runnerFor(tenant.MustStudentID(ctx))
	if err != nil {
		return nil, err
	}
	return extract_pipeline.ExtractTRPC(ctx, r.models.Chat, r.models.ChatMC, doc)
}

// GenerateReport 围绕某篇论文按类型生成研读报告,由前端按钮触发,不经分类器。
func GenerateReport(ctx context.Context, paperID string, t constant.ReportType) (*localagent.Reply, error) {
	owner := tenant.MustStudentID(ctx)
	r, err := runnerFor(owner)
	if err != nil {
		return nil, err
	}
	return report_pipeline.GenerateReportTRPC(ctx, r.models.Chat, r.models.ChatMC, &localagent.ReportInput{
		PaperID:    paperID,
		OwnerID:    owner,
		ReportType: t,
	})
}
