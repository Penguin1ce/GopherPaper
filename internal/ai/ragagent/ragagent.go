// Package ragagent 是论文助教 summary/method 两类问答的 agentic 生成入口。
// 与 agentrt(固定单轮生成)不同:这里挂 React planner,让 agent 在一个主循环里自主
// 规划→检索→反思→决策——用 search_paper / find_figures 工具按需多轮检索、判断召回是否充分,
// 而非由编排层预检索一次塞进 prompt。事实定位问题已并入 summary 走这里。
//
// 按 (userID, maxIter) 懒建并缓存 runner:模型取 aimodel 的 chat 模型,工具迭代上限按意图预算
// (summary/method 不同),故复合键缓存。历史走 WithInjectedContextMessages 注入、不持久化
// (SoR 仍是 history 包),runner 配 noop session;检索工具与出处收集见 ai/ragtools 叶子包。
package ragagent

import (
	"context"
	"fmt"
	"sync"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/planner/react"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	sessnoop "trpc.group/trpc-go/trpc-agent-go/session/noop"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/planstream"
	"GopherPaper/internal/ai/ragtools"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/tenant"
)

const (
	appName   = "gopherpaper"
	agentName = "paper-rag"
	sessionID = "default" // noop session 不落库,固定即可
)

// Policy 是意图分类后传给 agentic 循环的策略,目前只含工具迭代硬上限(预算)。
// 后续可扩展召回宽度、是否补邻近块等(见 TODO 的 RAG 优化项)。
type Policy struct {
	MaxIter int
}

// runners 按 (userID, maxIter) 缓存 runner:不同意图预算不同,故复合键。
var runners sync.Map // "userID|maxIter" -> *runnerEntry

type runnerEntry struct {
	once sync.Once
	rt   runner.Runner
	err  error
}

// Generate 经该用户的 agentic 问答 agent 跑一轮主循环并聚合成最终答案。
// instruction 为本轮 system prompt(经 AgenticRAGPromptFor 取),history 为多轮上下文(注入不持久化),
// query 为当前输入,policy 限定工具迭代预算。计划/检索/反思阶段经 planstream 推给前端计划面板。
func Generate(ctx context.Context, instruction string, history []trpcmodel.Message, query string, policy Policy) (string, error) {
	userID := tenant.MustStudentID(ctx)
	rt, err := runnerForUser(userID, policy.MaxIter)
	if err != nil {
		return "", err
	}
	ch, err := rt.Run(ctx, userID, sessionID, trpcmodel.NewUserMessage(query),
		agent.WithInstruction(instruction),
		agent.WithInjectedContextMessages(history),
	)
	if err != nil {
		return "", err
	}
	return planstream.CollectEvents(ctx, ch)
}

// runnerForUser 懒建该用户在某迭代预算下的 runner,模型取自 aimodel,工具为本包的检索工具。
func runnerForUser(userID string, maxIter int) (runner.Runner, error) {
	if userID == "" {
		return nil, fmt.Errorf("ragagent: userID 不能为空")
	}
	models, err := aimodel.ModelsForUser(userID)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s|%d", userID, maxIter)
	e, _ := runners.LoadOrStore(key, &runnerEntry{})
	ent := e.(*runnerEntry)
	ent.once.Do(func() {
		gc := core.GenConfig(models.ChatMC)
		// 带工具时部分 openai 兼容端点不能同用 reasoning_effort(400),剥离走端点默认;沿用 agentrt/pioneer。
		gc.ReasoningEffort = nil
		// 流式拉取:经 ctx 的 StreamHandler 把工具调用与规划/正文增量推给前端。
		gc.Stream = true
		opts := []llmagent.Option{
			llmagent.WithModel(models.Chat),
			llmagent.WithGenerationConfig(gc),
			// React planner:先规划再分步检索,输出按 PLANNING/ACTION/REASONING/REPLANNING/
			// FINAL_ANSWER 分段,由 planstream 分流给前端计划面板与正文。
			llmagent.WithPlanner(react.New()),
			// 工具迭代硬上限:防 agent 在检索循环里失控,软预算另在 prompt 引导。
			llmagent.WithMaxToolIterations(maxIter),
			llmagent.WithTools(ragtools.All()),
		}
		ent.rt = runner.NewRunner(appName, llmagent.New(agentName, opts...),
			runner.WithSessionService(sessnoop.NewService()))
	})
	return ent.rt, ent.err
}

// EvictUser 清除该用户缓存的全部 ragagent runner(各迭代预算下),登出时由 ai.EvictUser 调用。
// 检索工具是无状态包级构造、noop session 无持久态,下次访问 runnerForUser 重建。
func EvictUser(userID string) {
	if userID == "" {
		return
	}
	prefix := userID + "|"
	runners.Range(func(k, _ any) bool {
		if ks, ok := k.(string); ok && len(ks) >= len(prefix) && ks[:len(prefix)] == prefix {
			runners.Delete(k)
		}
		return true
	})
}

func EvictAll() {
	runners.Range(func(k, _ any) bool {
		runners.Delete(k)
		return true
	})
}
