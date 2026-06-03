// Package ai 编排助教的多 agent 路径，运行器与函数均为包级。
//
//	聊天 Chat: Host 使用 API 模型做 tool call 路由，只分发到 concept/debug/review 专家。
//	出题 GenerateExam、批改 Grade: 显式接口带结构化参数直接调对应 agent。
package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	einoagent "github.com/cloudwego/eino/flow/agent"
	hostma "github.com/cloudwego/eino/flow/agent/multiagent/host"
	"github.com/cloudwego/eino/schema"

	localagent "GopherCPP/internal/agent"
	"GopherCPP/internal/agent/chat_pipeline"
	"GopherCPP/internal/agent/exam_pipeline"
	"GopherCPP/internal/agent/grade_pipeline"
	"GopherCPP/pkg/constant"
)

// 编译好的路径，可并发复用，由 Init 初始化。
var (
	chatRunner  *hostma.MultiAgent
	examRunner  compose.Runnable[*localagent.AgentInput, *localagent.Reply]
	gradeRunner compose.Runnable[*localagent.AgentInput, *localagent.Reply]
)

// Init 组装并编译多 agent。chatModel 同时作为 Host 路由模型和下游专家模型。
func Init(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
) error {
	if err := chat_pipeline.Init(ctx); err != nil {
		return fmt.Errorf("init rag retriever: %w", err)
	}
	ragGraph, err := chat_pipeline.Build(chatModel)
	if err != nil {
		return fmt.Errorf("build rag agent: %w", err)
	}
	ragR, err := ragGraph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("compile rag agent: %w", err)
	}

	examGraph, err := exam_pipeline.Build(chatModel)
	if err != nil {
		return fmt.Errorf("build exam agent: %w", err)
	}
	examR, err := examGraph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("compile exam agent: %w", err)
	}

	gradeGraph, err := grade_pipeline.Build(chatModel)
	if err != nil {
		return fmt.Errorf("build grade agent: %w", err)
	}
	gradeR, err := gradeGraph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("compile grade agent: %w", err)
	}

	chat, err := buildChat(ctx, chatModel, ragR)
	if err != nil {
		return err
	}

	chatRunner = chat
	examRunner = examR
	gradeRunner = gradeR
	return nil
}

// buildChat 构造 Host Multi-Agent。Host 只负责选择一个专家，不直接回答。
func buildChat(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
	ragRunner compose.Runnable[*localagent.AgentInput, *localagent.Reply],
) (*hostma.MultiAgent, error) {
	hostAgent, err := hostma.NewMultiAgent(ctx, &hostma.MultiAgentConfig{
		Name: "gophercpp assistant host",
		Host: hostma.Host{
			ToolCallingModel: chatModel,
			SystemPrompt:     constant.HostPrompt,
		},
		Specialists: []*hostma.Specialist{
			ragSpecialist("concept_tutor", constant.IntentConcept, "讲解 C/C++ 概念、语法、原理、用法、标准库和最小示例。", ragRunner),
			ragSpecialist("debug_tutor", constant.IntentDebug, "定位 C/C++ 编译错误、运行时异常、崩溃、未定义行为和修复方案。", ragRunner),
			ragSpecialist("code_reviewer", constant.IntentReview, "评审、优化、重构学生给出的 C/C++ 代码，给出可执行改进建议。", ragRunner),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("build host multi-agent: %w", err)
	}
	return hostAgent, nil
}

// Chat 是聊天入口，ctx 须已注入租户信息供 RAG 检索隔离。
func Chat(ctx context.Context, query string) (*localagent.Reply, error) {
	if chatRunner == nil {
		return nil, fmt.Errorf("assistant chat runner 未初始化")
	}
	out, err := chatRunner.Generate(ctx, []*schema.Message{schema.UserMessage(query)})
	if err != nil {
		return nil, err
	}
	return messageToReply(out), nil
}

// GenerateExam 按结构化参数出题，由前端按钮触发。
func GenerateExam(ctx context.Context, topic, count, difficulty string) (*localagent.Reply, error) {
	in := &localagent.AgentInput{
		Query: topic,
		Intent: localagent.Intent{
			Type: constant.IntentExam,
			Slots: map[string]string{
				"topic":      topic,
				"count":      count,
				"difficulty": difficulty,
			},
		},
	}
	return examRunner.Invoke(ctx, in)
}

// Grade 批改学生作答，由前端按钮触发。submission 为题目与作答拼成的文本。
func Grade(ctx context.Context, submission string) (*localagent.Reply, error) {
	in := &localagent.AgentInput{
		Query:  submission,
		Intent: localagent.Intent{Type: constant.IntentGrade},
	}
	return gradeRunner.Invoke(ctx, in)
}

func ragSpecialist(
	name string,
	intent constant.IntentType,
	intendedUse string,
	runner compose.Runnable[*localagent.AgentInput, *localagent.Reply],
) *hostma.Specialist {
	return agentSpecialist(name, intent, intendedUse, runner, basicInput(intent))
}

func agentSpecialist(
	name string,
	intent constant.IntentType,
	intendedUse string,
	runner compose.Runnable[*localagent.AgentInput, *localagent.Reply],
	inputFn func(string) *localagent.AgentInput,
) *hostma.Specialist {
	return &hostma.Specialist{
		AgentMeta: hostma.AgentMeta{
			Name:        name,
			IntendedUse: intendedUse,
		},
		Invokable: func(ctx context.Context, msgs []*schema.Message, _ ...einoagent.AgentOption) (*schema.Message, error) {
			query := lastUserContent(msgs)
			in := inputFn(query)
			if in.Intent.Type == "" {
				in.Intent.Type = intent
			}
			reply, err := runner.Invoke(ctx, in)
			if err != nil {
				return nil, err
			}
			return replyToMessage(reply), nil
		},
	}
}

func basicInput(intent constant.IntentType) func(string) *localagent.AgentInput {
	return func(query string) *localagent.AgentInput {
		return &localagent.AgentInput{
			Query: query,
			Intent: localagent.Intent{
				Type:  intent,
				Slots: map[string]string{},
			},
		}
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
	reply := &localagent.Reply{Intent: constant.IntentConcept}
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
