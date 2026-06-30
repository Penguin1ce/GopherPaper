// Package gopher 是小囊鼠 agent:研读报告的 agentic RAG 生成器。
// 与论文助教的固定单轮不同,小囊鼠按 userID 懒建一个带检索工具(ragtools)、React planner 与
// report-research skill 的 runner:agent 自主多轮检索论文证据,按 skill 规定的流程逐方面补全后
// 撰写结构化报告。规划/检索/思考阶段经 planstream 推前端执行计划,出处经 ctx 收集器汇成 sources。
// 报告类型只换 prompt 里的聚焦点,不同类型复用同一 runner。
package gopher

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/chainagent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/planner/react"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	sessnoop "trpc.group/trpc-go/trpc-agent-go/session/noop"
	"trpc.group/trpc-go/trpc-agent-go/tool"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/planstream"
	"GopherPaper/internal/ai/ragtools"
	"GopherPaper/internal/ai/retrieval"
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
	err  error
}

// Generate 围绕某篇论文按类型经小囊鼠 agentic RAG 链路生成研读报告。
// owner 从 ctx 的 tenant 取;ctx 注入 paperID 让检索工具限定到本篇,挂出处收集器汇 sources。
func Generate(ctx context.Context, in *core.ReportInput) (*core.Reply, error) {
	userID := tenant.MustStudentID(ctx)
	rt, err := runnerForUser(userID)
	if err != nil {
		return nil, err
	}
	ctx = core.WithPaperID(ctx, in.PaperID)
	ctx = retrieval.WithRefSink(ctx)

	focus := constant.ReportFocusFor(in.ReportType)
	query := reportQuery(focus)

	ch, err := rt.Run(ctx, userID, sessionID, trpcmodel.NewUserMessage(query))
	if err != nil {
		return nil, err
	}
	content, err := planstream.CollectEvents(ctx, ch)
	if err != nil {
		return nil, err
	}
	content = sanitizeReport(content)

	reply := &core.Reply{
		Content: content,
		Meta:    map[string]any{"report_type": string(in.ReportType)},
	}
	if sources := retrieval.DrainRefs(ctx); len(sources) > 0 {
		reply.Meta["sources"] = sources
	}
	return reply, nil
}

// runnerForUser 懒建该用户的小囊鼠 runner:模型取 aimodel 的 chat 模型,挂 ragtools 检索工具、
// React planner 与 gopher 分组 skill,工具迭代预算放到报告级别。
func runnerForUser(userID string) (runner.Runner, error) {
	if userID == "" {
		return nil, fmt.Errorf("gopher: userID 不能为空")
	}
	models, err := aimodel.ModelsForUser(userID)
	if err != nil {
		return nil, err
	}
	e, _ := runners.LoadOrStore(userID, &runnerEntry{})
	ent := e.(*runnerEntry)
	ent.once.Do(func() {
		gc := core.GenConfig(models.ChatMC)
		// 带工具时部分 openai 兼容端点不能同用 reasoning_effort(400),剥离走端点默认;沿用 ragagent。
		gc.ReasoningEffort = nil
		// 流式拉取:经 ctx 的 StreamHandler 把规划/检索/思考阶段与正文增量推给前端。
		gc.Stream = true
		// 报告是长篇输出,端点默认温度(doubao≈1.0)在多轮长上下文下易采样退化:吐垃圾串、
		// 自我批判、甚至重写出第二份报告。压低温度并加 frequency_penalty 抑制重复跑飞,只作用本链路。
		gc.Temperature = trpcmodel.Float64Ptr(constant.ReportTemperature)
		gc.FrequencyPenalty = trpcmodel.Float64Ptr(constant.ReportFrequencyPenalty)
		researcherOpts := []llmagent.Option{
			llmagent.WithModel(models.Chat),
			llmagent.WithGenerationConfig(gc),
			// React planner:先规划再分步检索,输出按 PLANNING/ACTION/REASONING/REPLANNING/
			// FINAL_ANSWER 分段,由 planstream 分流给前端执行计划与正文。
			llmagent.WithPlanner(react.New()),
			// 报告要覆盖全文、按结构逐方面检索,迭代预算给得比问答更宽。
			llmagent.WithMaxToolIterations(constant.AgenticMaxIterReport),
			llmagent.WithTools(ragtools.All()),
			llmagent.WithInstruction(constant.GopherResearcherPrompt),
		}
		if repo := toolkit.SkillRepoFor(constant.AgentGopher); repo != nil {
			researcherOpts = append(researcherOpts, llmagent.WithSkills(repo))
		}
		writerGC := gc
		writerGC.ReasoningEffort = core.GenConfig(models.ChatMC).ReasoningEffort
		writerOpts := []llmagent.Option{
			llmagent.WithModel(models.Chat),
			llmagent.WithGenerationConfig(writerGC),
			llmagent.WithInstruction(constant.GopherWriterPrompt),
		}
		reviewerGC := writerGC
		reviewerGC.Temperature = trpcmodel.Float64Ptr(constant.ReportReviewTemperature)
		reviewerOpts := []llmagent.Option{
			llmagent.WithModel(models.Chat),
			llmagent.WithGenerationConfig(reviewerGC),
			llmagent.WithInstruction(constant.GopherReviewerPrompt),
		}
		pipeline := chainagent.New(constant.AgentGopher, chainagent.WithSubAgents([]agent.Agent{
			newReportStageAgent(
				llmagent.New("gopher-researcher", researcherOpts...),
				constant.ReportPhaseResearching,
				"找资料:小囊鼠正在检索论文片段、图表和出处。",
			),
			newReportStageAgent(
				llmagent.New("gopher-writer", writerOpts...),
				constant.ReportPhaseWriting,
				"写报告:正在把证据笔记整理成结构化研读报告。",
			),
			newReportStageAgent(
				llmagent.New("gopher-reviewer", reviewerOpts...),
				constant.ReportPhaseReviewing,
				"评审:正在核对事实依据、补齐出处并压实结论。",
			),
		}))
		ent.rt = runner.NewRunner(appName, pipeline,
			runner.WithSessionService(sessnoop.NewService()))
	})
	return ent.rt, ent.err
}

type reportStageAgent struct {
	inner agent.Agent
	phase string
	text  string
}

func newReportStageAgent(inner agent.Agent, phase, text string) agent.Agent {
	return &reportStageAgent{inner: inner, phase: phase, text: text}
}

func (a *reportStageAgent) Run(ctx context.Context, invocation *agent.Invocation) (<-chan *event.Event, error) {
	emitReportPhase(ctx, a.phase, a.text)
	return a.inner.Run(ctx, invocation)
}

func (a *reportStageAgent) Tools() []tool.Tool {
	return a.inner.Tools()
}

func (a *reportStageAgent) Info() agent.Info {
	return a.inner.Info()
}

func (a *reportStageAgent) SubAgents() []agent.Agent {
	return a.inner.SubAgents()
}

func (a *reportStageAgent) FindSubAgent(name string) agent.Agent {
	return a.inner.FindSubAgent(name)
}

func emitReportPhase(ctx context.Context, phase, text string) {
	if stream := core.StreamFrom(ctx); stream != nil {
		stream(core.StreamEvent{Kind: constant.StreamEventPlan, Phase: phase, Delta: text})
	}
}

// EvictUser 清除该用户缓存的小囊鼠 runner,登出时调用。下次访问 runnerForUser 重建。
func EvictUser(userID string) {
	runners.Delete(userID)
}

// reportH1 匹配 Markdown 一级标题行。一份报告至多一个一级标题,出现第二个即模型跑飞重写了第二份。
var reportH1 = regexp.MustCompile(`(?m)^#\s+\S`)

func reportQuery(focus string) string {
	brief := strings.ReplaceAll(constant.GopherReportPrompt, "{focus}", focus)
	return brief + "\n\n当前任务: 请按小囊鼠 researcher -> writer -> reviewer 的顺序完成本篇论文研读报告。最终交付只保留 reviewer 修订后的完整 Markdown 报告。"
}

// sanitizeReport 是报告输出的最后一道防线:采样退化时模型偶发在一轮里重写出第二份报告
// (前一份后跟思维链自语、垃圾串,再 # 重开一份)。检测到第二个一级标题即只保留第一份完整报告,
// 截掉其后的所有内容。采样参数(ReportTemperature/FrequencyPenalty)是根因治理,这里兜底残留。
func sanitizeReport(s string) string {
	if idx := strings.LastIndex(s, react.FinalAnswerTag); idx >= 0 {
		s = s[idx+len(react.FinalAnswerTag):]
	}
	for _, tag := range []string{
		react.PlanningTag,
		react.ReplanningTag,
		react.ActionTag,
		react.ReasoningTag,
		react.FinalAnswerTag,
	} {
		s = strings.ReplaceAll(s, tag, "")
	}
	locs := reportH1.FindAllStringIndex(s, -1)
	if len(locs) >= 2 {
		s = s[:locs[1][0]]
	}
	return strings.TrimSpace(s)
}
