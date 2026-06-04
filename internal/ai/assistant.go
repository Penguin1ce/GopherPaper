// Package ai 编排论文问答的多 agent 路径，运行器按 userID 懒编译并缓存。
//
//	问答 Chat: Host 用该用户的意图模型做 tool call 路由，分发到 fact/summary/method 专家。
//	抽取 Extract、报告 GenerateReport: 显式接口直接调对应 agent。
//
// 因为模型按用户单例,Host multi-agent 与各 pipeline runner 内嵌模型,编排也随之每用户化。
// 与用户无关的全局检索器由 Init 一次性准备。
package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	einoagent "github.com/cloudwego/eino/flow/agent"
	hostma "github.com/cloudwego/eino/flow/agent/multiagent/host"
	"github.com/cloudwego/eino/schema"

	localagent "GopherPaper/internal/agent"
	"GopherPaper/internal/agent/chat_pipeline"
	"GopherPaper/internal/agent/extract_pipeline"
	"GopherPaper/internal/agent/report_pipeline"
	"GopherPaper/internal/factory"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
)

// userRunner 是单个用户编译好的全套 agent 运行器,可并发复用。
type userRunner struct {
	chat    *hostma.MultiAgent
	report  compose.Runnable[*localagent.ReportInput, *localagent.Reply]
	extract compose.Runnable[*localagent.ParsedDoc, *localagent.PaperStructured]
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

// Init 只做与用户无关的一次性准备:构建全局检索器。模型与 runner 按用户懒编译。
func Init(ctx context.Context) error {
	if err := chat_pipeline.Init(ctx); err != nil {
		return fmt.Errorf("init rag retriever: %w", err)
	}
	ready = true
	return nil
}

// runnerFor 返回某用户的运行器,未命中则用该用户模型编译并缓存。
func runnerFor(ctx context.Context, userID string) (*userRunner, error) {
	if !ready {
		return nil, fmt.Errorf("ai: 编排器未初始化")
	}
	if userID == "" {
		return nil, fmt.Errorf("ai: 缺少用户身份")
	}
	e, _ := userRunners.LoadOrStore(userID, &runnerEntry{})
	ent := e.(*runnerEntry)
	ent.once.Do(func() {
		ent.runner, ent.err = buildUserRunner(ctx, userID)
	})
	if ent.err != nil {
		userRunners.Delete(userID)
		return nil, ent.err
	}
	return ent.runner, nil
}

func buildUserRunner(ctx context.Context, userID string) (*userRunner, error) {
	um, err := factory.ModelsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	ragGraph, err := chat_pipeline.Build(um.Chat)
	if err != nil {
		return nil, fmt.Errorf("build rag agent: %w", err)
	}
	ragR, err := ragGraph.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("compile rag agent: %w", err)
	}

	reportGraph, err := report_pipeline.Build(um.Chat)
	if err != nil {
		return nil, fmt.Errorf("build report agent: %w", err)
	}
	reportR, err := reportGraph.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("compile report agent: %w", err)
	}

	extractGraph, err := extract_pipeline.Build(um.Chat)
	if err != nil {
		return nil, fmt.Errorf("build extract agent: %w", err)
	}
	extractR, err := extractGraph.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("compile extract agent: %w", err)
	}

	chat, err := buildChat(ctx, um.Intent, ragR)
	if err != nil {
		return nil, err
	}
	return &userRunner{chat: chat, report: reportR, extract: extractR}, nil
}

// buildChat 构造 Host Multi-Agent。Host 只负责选择一个专家,不直接回答。
func buildChat(
	ctx context.Context,
	hostModel model.ToolCallingChatModel,
	ragRunner compose.Runnable[*localagent.AgentInput, *localagent.Reply],
) (*hostma.MultiAgent, error) {
	hostAgent, err := hostma.NewMultiAgent(ctx, &hostma.MultiAgentConfig{
		Name: "research literature assistant host",
		Host: hostma.Host{
			ToolCallingModel: hostModel,
			SystemPrompt:     constant.HostPrompt,
		},
		Specialists: []*hostma.Specialist{
			ragSpecialist("fact_expert", constant.IntentFact, "定位论文中的具体事实、数据、结论、数值。", ragRunner),
			ragSpecialist("summary_expert", constant.IntentSummary, "概括、解释、综述论文整体或某部分内容。", ragRunner),
			ragSpecialist("method_expert", constant.IntentMethod, "解读研究方法、实验设计、技术流程与步骤。", ragRunner),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("build host multi-agent: %w", err)
	}
	return hostAgent, nil
}

// Chat 是问答入口,history 为按时间升序的历史消息,query 为本轮提问。
// ctx 须已注入租户信息供 RAG 检索隔离;Slots["paper_id"] 由调用方在 query 外另行注入到检索。
func Chat(ctx context.Context, history []*schema.Message, query string) (*localagent.Reply, error) {
	r, err := runnerFor(ctx, tenant.MustStudentID(ctx))
	if err != nil {
		return nil, err
	}
	msgs := make([]*schema.Message, 0, len(history)+1)
	msgs = append(msgs, history...)
	msgs = append(msgs, schema.UserMessage(query))
	out, err := r.chat.Generate(ctx, msgs)
	if err != nil {
		return nil, err
	}
	return messageToReply(out), nil
}

// Extract 把解析后的论文抽成结构化信息,由 parse worker 调用。ctx 须注入论文 owner。
func Extract(ctx context.Context, doc *localagent.ParsedDoc) (*localagent.PaperStructured, error) {
	r, err := runnerFor(ctx, tenant.MustStudentID(ctx))
	if err != nil {
		return nil, err
	}
	return r.extract.Invoke(ctx, doc)
}

// GenerateReport 围绕某篇论文按类型生成研读报告,由前端按钮触发,不经分类器。
func GenerateReport(ctx context.Context, paperID string, t constant.ReportType) (*localagent.Reply, error) {
	owner := tenant.MustStudentID(ctx)
	r, err := runnerFor(ctx, owner)
	if err != nil {
		return nil, err
	}
	return r.report.Invoke(ctx, &localagent.ReportInput{
		PaperID:    paperID,
		OwnerID:    owner,
		ReportType: t,
	})
}

func ragSpecialist(
	name string,
	intent constant.IntentType,
	intendedUse string,
	runner compose.Runnable[*localagent.AgentInput, *localagent.Reply],
) *hostma.Specialist {
	return &hostma.Specialist{
		AgentMeta: hostma.AgentMeta{
			Name:        name,
			IntendedUse: intendedUse,
		},
		Invokable: func(ctx context.Context, msgs []*schema.Message, _ ...einoagent.AgentOption) (*schema.Message, error) {
			in := &localagent.AgentInput{
				Query:  lastUserContent(msgs),
				Intent: localagent.Intent{Type: intent, Slots: map[string]string{}},
			}
			reply, err := runner.Invoke(ctx, in)
			if err != nil {
				return nil, err
			}
			return replyToMessage(reply), nil
		},
	}
}

func lastUserContent(msgs []*schema.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i] != nil && msgs[i].Role == schema.User && strings.TrimSpace(msgs[i].Content) != "" {
			return msgs[i].Content
		}
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i] != nil && strings.TrimSpace(msgs[i].Content) != "" {
			return msgs[i].Content
		}
	}
	return ""
}

func replyToMessage(reply *localagent.Reply) *schema.Message {
	msg := schema.AssistantMessage(reply.Content, nil)
	msg.Extra = map[string]any{"intent": string(reply.Intent)}
	if reply.Meta != nil {
		msg.Extra["meta"] = reply.Meta
	}
	return msg
}

func messageToReply(msg *schema.Message) *localagent.Reply {
	reply := &localagent.Reply{Intent: constant.IntentSummary}
	if msg == nil {
		return reply
	}
	reply.Content = msg.Content
	if msg.Extra == nil {
		return reply
	}
	if intentName, ok := msg.Extra["intent"].(string); ok && intentName != "" {
		reply.Intent = constant.IntentType(intentName)
	}
	if meta, ok := msg.Extra["meta"].(map[string]any); ok {
		reply.Meta = meta
	}
	return reply
}
