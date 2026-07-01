// Package pioneer 是小云雀 agent:面向用户的多面手助手,与论文问答的 RAG 链路解耦。
// 按 userID 懒建并缓存一个带工具的 llmagent+runner,挂 pioneer 分组的 mcp 工具与 skill。
// 凭据型工具(如瑞幸点单)的 token 由请求 ctx 携带,经 toolkit 的钩子按调用注入,服务端不落库。
// 跨轮记忆走 runner 的真实 Redis session(按 userID+sessionID 隔离),承载完整 ReAct 轨迹
// (含工具调用与返回),上轮工具结果下轮可复用,减少重复调用;用 MaxHistoryRuns 限上下文增长。
// 工作记忆存 Redis 故跨进程重启不丢,闲置按 TTL 自动回收,不压业务 MySQL;键前缀与展示历史分库。
// 历史展示的 system-of-record 仍是 history 包(MySQL,只存干净文本),与这份工作记忆分工不同。
package pioneer

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	trpcevent "trpc.group/trpc-go/trpc-agent-go/event"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/planner/react"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	trpcsession "trpc.group/trpc-go/trpc-agent-go/session"
	redissession "trpc.group/trpc-go/trpc-agent-go/session/redis"
	"trpc.group/trpc-go/trpc-agent-go/session/summary"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/planstream"
	"GopherPaper/internal/ai/toolkit"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/config"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
)

const appName = "gopherpaper"

// runners 按 userID 缓存 runner,与 aimodel 的模型缓存一一对应。
var runners sync.Map // userID -> *runnerEntry

// sessStore 是小云雀的会话工作记忆:Redis session,按 (userID, sessionID) 隔离,
// 承载完整 ReAct 轨迹供跨轮复用。由 Init 建好,键前缀与展示历史分库、空闲按 TTL 回收;
// 存 Redis 故跨进程重启不丢(工作记忆属优化,真要清理直接删 Redis 键即可)。
var sessStore trpcsession.Service

// Init 用 Redis 配置建小云雀工作记忆的 session 服务,并挂上会话摘要器做多轮上下文治理。
// 须在配置加载后调用,先于首轮 Chat。chatCfg 是全局 chat 模型配置,供摘要器生成摘要。
func Init(cfg config.RedisConfig, chatCfg config.ModelConfig) error {
	// 会话摘要器:对话累积到阈值后把较早轮次压缩成摘要,做上下文治理。
	// 模型用全局 chat 模型(所有用户同一把 key),故一个全局 summarizer 即可复用。
	summarizer := summary.NewSummarizer(
		aimodel.NewChatModel(chatCfg),
		summary.WithEventThreshold(constant.PioneerSummaryEventThreshold),
		summary.WithMaxSummaryWords(constant.PioneerSummaryMaxWords),
		summary.WithPrompt(constant.PioneerSummaryPrompt),
		// 跳过最近若干事件不进摘要,保证刚摘要完仍有「最近原文」留底。
		summary.WithSkipRecent(func([]trpcevent.Event) int { return constant.PioneerSummarySkipRecentEvents }),
	)
	s, err := redissession.NewService(
		redissession.WithRedisClientURL(redisURL(cfg)),
		redissession.WithKeyPrefix(constant.PioneerSessionKeyPrefix),
		redissession.WithSessionTTL(constant.PioneerSessionTTL),
		// 挂摘要器:runner 每轮结束后自动按阈值入队异步摘要,后台生成不阻断回答。
		redissession.WithSummarizer(summarizer),
		redissession.WithAsyncSummaryNum(constant.PioneerSummaryAsyncWorkers),
		redissession.WithSummaryQueueSize(constant.PioneerSummaryQueueSize),
		redissession.WithSummaryJobTimeout(constant.PioneerSummaryJobTimeout),
	)
	if err != nil {
		return fmt.Errorf("pioneer: 初始化 Redis session 失败: %w", err)
	}
	sessStore = s
	return nil
}

// redisURL 把 RedisConfig 拼成 redis://[:password@]addr/db,供 session 服务建独立连接池。
func redisURL(cfg config.RedisConfig) string {
	u := url.URL{Scheme: "redis", Host: cfg.Addr, Path: fmt.Sprintf("/%d", cfg.DB)}
	if cfg.Password != "" {
		u.User = url.UserPassword("", cfg.Password)
	}
	return u.String()
}

type runnerEntry struct {
	once sync.Once
	rt   runner.Runner
}

// Chat 经该用户的小云雀 agent 跑一轮并聚合成完整文本。
// 多轮上下文由 runner 的 Redis session 按 sessionID 自动承载(含工具轨迹),无需手工注入历史。
// query 为当前输入,用户身份从 ctx 的 tenant 取。
func Chat(ctx context.Context, sessionID, query string) (string, error) {
	if sessStore == nil {
		return "", fmt.Errorf("pioneer: session 未初始化")
	}
	userID := tenant.MustStudentID(ctx)
	rt, err := runnerForUser(userID)
	if err != nil {
		return "", err
	}
	ch, err := rt.Run(ctx, userID, sessionID, trpcmodel.NewUserMessage(query),
		agent.WithInstruction(withUserPreference(ctx, constant.PioneerRuntimeInstruction())),
	)
	if err != nil {
		return "", err
	}
	// 挂了 React planner,模型输出带规划/动作标签,用标签感知收集器分流 plan 与正文。
	return planstream.CollectEvents(ctx, ch)
}

func withUserPreference(ctx context.Context, instruction string) string {
	if pref := strings.TrimSpace(core.UserPreferenceFrom(ctx)); pref != "" {
		return instruction + "\n\n" + pref
	}
	return instruction
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
			// 多轮上下文治理:把分支摘要作为 system 消息前置,摘要水位线之后保留原文。
			// 开此项后 MaxHistoryRuns 由摘要水位线接管(框架忽略),早期上下文经摘要不丢。
			llmagent.WithAddSessionSummary(true),
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
			runner.WithSessionService(sessStore))
	})
	return ent.rt, nil
}

// EvictUser 清除该用户缓存的小云雀 runner,登出时调用释放常驻内存。
// 工具连接由 toolkit 按 token 独立回收、工作记忆在共享 Redis session(按 sessionID 隔离、
// 按 TTL 回收),均不随 runner 走;下次访问 runnerForUser 重建。
func EvictUser(userID string) {
	runners.Delete(userID)
}
