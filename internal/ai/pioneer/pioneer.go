// Package pioneer 是小云雀 agent:面向用户的多面手助手,与论文问答的 RAG 链路解耦。
// 按 userID 懒建并缓存一个带工具的 llmagent+runner,挂 pioneer 分组的 mcp 工具与 skill。
// 凭据型工具(如瑞幸点单)的 token 由请求 ctx 携带,经 toolkit 的钩子按调用注入,服务端不落库。
// 历史按请求注入不持久化,system-of-record 仍是 history 包,runner 配 noop session。
package pioneer

import (
	"context"
	"fmt"
	"sync"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	sessnoop "trpc.group/trpc-go/trpc-agent-go/session/noop"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/toolkit"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
)

const (
	appName   = "gopherpaper"
	sessionID = "default" // noop session 不落库,固定即可
)

// runners 按 userID 缓存 runner,与 aimodel 的模型缓存一一对应。
var runners sync.Map // userID -> *runnerEntry

type runnerEntry struct {
	once sync.Once
	rt   runner.Runner
}

// Chat 经该用户的小云雀 agent 跑一轮并聚合成完整文本。
// history 为多轮上下文(注入不持久化),query 为当前输入,用户身份从 ctx 的 tenant 取。
func Chat(ctx context.Context, history []trpcmodel.Message, query string) (string, error) {
	userID := tenant.MustStudentID(ctx)
	rt, err := runnerForUser(userID)
	if err != nil {
		return "", err
	}
	ch, err := rt.Run(ctx, userID, sessionID, trpcmodel.NewUserMessage(query),
		agent.WithInstruction(constant.PioneerInstruction),
		agent.WithInjectedContextMessages(history),
	)
	if err != nil {
		return "", err
	}
	return core.CollectEvents(ctx, ch)
}

// runnerForUser 懒建该用户的 runner,模型取自 aimodel,工具与 skill 取 pioneer 分组。
func runnerForUser(userID string) (runner.Runner, error) {
	if userID == "" {
		return nil, fmt.Errorf("pioneer: userID 不能为空")
	}
	models, err := aimodel.ModelsForUser(userID)
	if err != nil {
		return nil, err
	}
	e, _ := runners.LoadOrStore(userID, &runnerEntry{})
	ent := e.(*runnerEntry)
	ent.once.Do(func() {
		// 兼容兜底:部分 openai 兼容端点在 chat/completions 下 function tools 与
		// reasoning_effort 不能同用(400),小云雀必带工具,故剥离推理强度走端点默认。
		gc := core.GenConfig(models.PioneerMC)
		gc.ReasoningEffort = nil
		// 流式拉取模型输出,经 ctx 的 StreamHandler 把工具调用与增量推给前端。
		gc.Stream = true
		opts := []llmagent.Option{
			llmagent.WithModel(models.Pioneer),
			llmagent.WithGenerationConfig(gc),
			// 工具列表在每轮运行时用请求 ctx 重建:凭据型工具集靠 ctx 里的 token
			// 才能过远端鉴权,构建期的 context.Background 拉不到(401)。
			llmagent.WithRefreshToolSetsOnRun(true),
		}
		if sets := toolkit.ToolSetsFor(constant.AgentPioneer); len(sets) > 0 {
			opts = append(opts, llmagent.WithToolSets(sets))
		}
		if ts := toolkit.ToolsFor(constant.AgentPioneer); len(ts) > 0 {
			opts = append(opts, llmagent.WithTools(ts))
		}
		if repo := toolkit.SkillRepoFor(constant.AgentPioneer); repo != nil {
			opts = append(opts, llmagent.WithSkills(repo))
		}
		ent.rt = runner.NewRunner(appName, llmagent.New(constant.AgentPioneer, opts...),
			runner.WithSessionService(sessnoop.NewService()))
	})
	return ent.rt, nil
}
