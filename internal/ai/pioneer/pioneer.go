// Package pioneer 是小云雀 agent:面向用户的多面手助手,与论文问答的 RAG 链路解耦。
// 按 userID 懒建并缓存一个带工具的 llmagent+runner,挂 pioneer 分组的 mcp 工具与 skill。
// 凭据型工具(如瑞幸点单)的 token 由请求 ctx 携带,经 toolkit 的钩子按调用注入,服务端不落库。
// 跨轮记忆走 runner 的真实 inmemory session(按 userID+sessionID 隔离),承载完整 ReAct 轨迹
// (含工具调用与返回),上轮工具结果下轮可复用,减少重复调用;用 MaxHistoryRuns 限上下文增长。
// 历史展示的 system-of-record 仍是 history 包(只存干净文本),与这份工作记忆分工不同。
package pioneer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/planner/react"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/toolkit"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
)

const appName = "gopherpaper"

// runners 按 userID 缓存 runner,与 aimodel 的模型缓存一一对应。
var runners sync.Map // userID -> *runnerEntry

// sessMem 是小云雀的会话工作记忆:进程内 inmemory session,按 (userID, sessionID) 隔离,
// 承载完整 ReAct 轨迹供跨轮复用。配 2 小时空闲 TTL + 自动清理防内存堆积;
// 进程重启或超时即丢(属优化非正确性,需持久化可后续换 mysql session)。
var sessMem = inmemory.NewSessionService(inmemory.WithSessionTTL(2 * time.Hour))

type runnerEntry struct {
	once sync.Once
	rt   runner.Runner
}

// Chat 经该用户的小云雀 agent 跑一轮并聚合成完整文本。
// 多轮上下文由 runner 的 inmemory session 按 sessionID 自动承载(含工具轨迹),无需手工注入历史。
// query 为当前输入,用户身份从 ctx 的 tenant 取。
func Chat(ctx context.Context, sessionID, query string) (string, error) {
	userID := tenant.MustStudentID(ctx)
	rt, err := runnerForUser(userID)
	if err != nil {
		return "", err
	}
	ch, err := rt.Run(ctx, userID, sessionID, trpcmodel.NewUserMessage(query),
		agent.WithInstruction(constant.PioneerInstruction),
	)
	if err != nil {
		return "", err
	}
	// 挂了 React planner,模型输出带规划/动作标签,用标签感知收集器分流 plan 与正文。
	return collectPlanEvents(ctx, ch)
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
			// React planner:先规划再分步执行工具,输出按 PLANNING/ACTION/REASONING/
			// FINAL_ANSWER 标签分段,由 collectPlanEvents 分流给前端计划面板与正文。
			llmagent.WithPlanner(react.New()),
			// 跨轮记忆:session 累积全量轨迹,只取最近若干条喂模型,防上下文无限膨胀;
			// 截断点落在孤儿 tool 结果上时框架自动跳过,不会触发 tool_use_id 报错。
			llmagent.WithMaxHistoryRuns(constant.PioneerMaxHistoryRuns),
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
			runner.WithSessionService(sessMem))
	})
	return ent.rt, nil
}
