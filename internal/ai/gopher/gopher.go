// Package gopher 是小囊鼠 agent:研读报告的 agentic RAG 生成器。
// 与论文助教的固定单轮不同,小囊鼠按 userID 懒建一个带检索工具(ragtools)、React planner 与
// report-research skill 的 runner:agent 自主多轮检索论文证据,按 skill 规定的流程逐方面补全后
// 撰写结构化报告。规划/检索/思考阶段经 planstream 推前端执行计划,出处经 ctx 收集器汇成 sources。
// 报告类型只换 prompt 里的聚焦点,不同类型复用同一 runner。
package gopher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/chainagent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/graph"
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
	"GopherPaper/internal/zlog"
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
		if errors.Is(err, planstream.ErrMaxToolIterations) {
			return fallbackReport(ctx, in, focus, err)
		}
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
	ch, err := a.inner.Run(ctx, invocation)
	if err != nil {
		return nil, err
	}
	if a.phase == constant.ReportPhaseResearching {
		return logResearcherFinalContent(ctx, ch), nil
	}
	return ch, nil
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

func logResearcherFinalContent(ctx context.Context, in <-chan *event.Event) <-chan *event.Event {
	out := make(chan *event.Event)
	go func() {
		defer close(out)
		var finalContent string
		for ev := range in {
			if content := researcherEventContent(ev); content != "" {
				finalContent = content
			}
			out <- ev
		}
		finalContent = strings.TrimSpace(finalContent)
		if finalContent == "" {
			zlog.Info("小囊鼠 researcher 最终内容为空", "paper_id", core.PaperIDFrom(ctx))
			return
		}
		zlog.Info("小囊鼠 researcher 最终内容",
			"paper_id", core.PaperIDFrom(ctx),
			"chars", len([]rune(finalContent)),
			"content", finalContent,
		)
	}()
	return out
}

func researcherEventContent(ev *event.Event) string {
	if ev == nil || ev.Response == nil || ev.Object == trpcmodel.ObjectTypeToolResponse {
		return ""
	}
	if content := strings.TrimSpace(responseContent(ev)); content != "" {
		return content
	}
	if raw := ev.StateDelta[graph.StateKeyLastResponse]; len(raw) > 0 {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func responseContent(ev *event.Event) string {
	if ev == nil || ev.Response == nil || ev.IsPartial {
		return ""
	}
	var b strings.Builder
	for _, c := range ev.Choices {
		b.WriteString(c.Message.Content)
	}
	return strings.TrimSpace(b.String())
}

// EvictUser 清除该用户缓存的小囊鼠 runner(含思路图),登出时调用。下次访问自动重建。
func EvictUser(userID string) {
	runners.Delete(userID)
	flowRunners.Delete(userID)
}

// reportH1 匹配 Markdown 一级标题行。一份报告至多一个一级标题,出现第二个即模型跑飞重写了第二份。
var reportH1 = regexp.MustCompile(`(?m)^#\s+\S`)

// reportQuery 生成链上三段共享的用户消息:只有任务简报与共享约束,不下达角色顺序指令。
// 顺序由 chainagent 结构保证,消息里写了角色流程会让挂 planner 的 researcher 把写作评审也规划进去。
func reportQuery(focus string) string {
	return strings.ReplaceAll(constant.GopherReportPrompt, "{focus}", focus)
}

const (
	fallbackDocsPerQuery = 3
	fallbackMaxTextDocs  = 18
)

func fallbackReport(ctx context.Context, in *core.ReportInput, focus string, cause error) (*core.Reply, error) {
	userID := tenant.MustStudentID(ctx)
	zlog.Warn("报告工具迭代耗尽,启用固定检索兜底",
		"paper_id", in.PaperID, "type", string(in.ReportType), "err", cause)
	emitReportPhase(ctx, constant.ReportPhaseWriting, "写报告:检索轮数已达上限,正在用固定检索结果生成保守报告。")

	docs := fallbackReportDocs(ctx, userID, in.PaperID, in.ReportType)
	prompt := strings.ReplaceAll(constant.GopherFallbackReportPrompt, "{focus}", focus)
	prompt = strings.ReplaceAll(prompt, "{context}", retrieval.FormatDocs(docs)+fallbackFigureInstruction(docs))

	models, err := aimodel.ModelsForUser(userID)
	if err != nil {
		return nil, err
	}
	gc := core.GenConfig(models.ChatMC)
	gc.Temperature = trpcmodel.Float64Ptr(constant.ReportTemperature)
	gc.FrequencyPenalty = trpcmodel.Float64Ptr(constant.ReportFrequencyPenalty)
	req := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(prompt),
			trpcmodel.NewUserMessage("请生成最终版研读报告。"),
		},
		GenerationConfig: gc,
	}
	content, err := core.GenerateText(ctx, models.Chat, req)
	if err != nil {
		return nil, fmt.Errorf("gopher: 兜底报告生成失败: %w", err)
	}
	content = sanitizeReport(content)

	reply := &core.Reply{
		Content: content,
		Meta: map[string]any{
			"report_type":     string(in.ReportType),
			"fallback":        true,
			"fallback_reason": "max_tool_iterations",
		},
	}
	if sources := retrieval.References(docs); len(sources) > 0 {
		reply.Meta["sources"] = sources
	}
	return reply, nil
}

func fallbackReportDocs(ctx context.Context, ownerID, paperID string, t constant.ReportType) []*retrieval.Doc {
	seen := map[string]struct{}{}
	docs := make([]*retrieval.Doc, 0, fallbackMaxTextDocs+constant.TopKImages)
	for _, q := range fallbackReportQueries(t) {
		got, err := retrieval.RetrieveForPaper(ctx, q, ownerID, paperID)
		if err != nil {
			zlog.Error("兜底报告正文检索失败", "paper_id", paperID, "query", q, "err", err)
			continue
		}
		docs = appendUniqueDocs(docs, seen, retrieval.DropImageDocs(got), fallbackDocsPerQuery, fallbackMaxTextDocs)
		if len(docs) >= fallbackMaxTextDocs {
			break
		}
	}
	if figs, err := retrieval.RetrieveImagesForPaper(ctx, fallbackFigureQuery(t), ownerID, paperID); err != nil {
		zlog.Error("兜底报告图表检索失败", "paper_id", paperID, "type", string(t), "err", err)
	} else {
		docs = appendUniqueDocs(docs, seen, figs, constant.TopKImages, fallbackMaxTextDocs+constant.TopKImages)
	}
	return docs
}

func appendUniqueDocs(out []*retrieval.Doc, seen map[string]struct{}, docs []*retrieval.Doc, maxAdd, maxTotal int) []*retrieval.Doc {
	added := 0
	for _, d := range docs {
		if d == nil {
			continue
		}
		if _, ok := seen[d.ID]; ok {
			continue
		}
		seen[d.ID] = struct{}{}
		out = append(out, d)
		added++
		if (maxAdd > 0 && added >= maxAdd) || (maxTotal > 0 && len(out) >= maxTotal) {
			break
		}
	}
	return out
}

func fallbackReportQueries(t constant.ReportType) []string {
	switch t {
	case constant.ReportMethod:
		return []string{
			"总体技术路线 模型架构 输入输出 流程",
			"关键模块 算法步骤 目标函数 训练策略 推理策略",
			"模块设计动机 前后步骤衔接",
			"实现细节 数据预处理 参数设置 模型规模",
			"方法优势 代价 复杂度 失败条件",
		}
	case constant.ReportResult:
		return []string{
			"主实验 数据集 指标 对比基线 结果",
			"ASR ACC ASRt 表格 提升幅度",
			"消融实验 模块有效性",
			"鲁棒性 泛化 分组分析",
			"失败案例 错误分析 适用范围",
		}
	case constant.ReportInnovation:
		return []string{
			"作者贡献 创新点 摘要 引言",
			"相关工作 差异 旧方法不足",
			"技术新意 架构 目标函数 数据构造 训练策略",
			"实验证据 支持创新点 证据强弱",
			"局限性 失败案例 适用边界 风险",
		}
	case constant.ReportFuture:
		return []string{
			"局限 future work 未解决问题",
			"薄弱环节 失败案例 泛化不足",
			"方法改进 模型结构 目标函数 数据 训练策略",
			"实验补充 更多数据集 更多基线 长期评估",
			"应用迁移 工程部署 成本 延迟 安全隐私",
		}
	default:
		return []string{
			"论文题目 研究动机 目标问题 痛点",
			"核心假设 关键挑战 问题定义",
			"方法总体思路 主要模块 推理流程",
			"实验设置 数据集 评测指标 主要结果",
			"贡献 创新点 局限性 future work",
		}
	}
}

func fallbackFigureQuery(t constant.ReportType) string {
	switch t {
	case constant.ReportMethod:
		return "architecture workflow method diagram pipeline"
	case constant.ReportResult:
		return "main results table ablation curve metrics"
	case constant.ReportInnovation:
		return "contribution comparison limitation result table"
	case constant.ReportFuture:
		return "limitation failure case future work result table"
	default:
		return "overview architecture main results table figure"
	}
}

func fallbackFigureInstruction(docs []*retrieval.Doc) string {
	var b strings.Builder
	for _, d := range docs {
		ref := retrieval.ReferenceFromDocument(d)
		if ref.ImgName == "" {
			continue
		}
		if b.Len() == 0 {
			b.WriteString("\n\n可用图表占位如下,只有正文需要时才插入:\n")
		}
		if ref.CitationTag != "" {
			fmt.Fprintf(&b, "- figure://%s : %s, citation_tag: %s\n", ref.ImgName, retrieval.FormatReference(ref), ref.CitationTag)
		} else {
			fmt.Fprintf(&b, "- figure://%s : %s\n", ref.ImgName, retrieval.FormatReference(ref))
		}
	}
	return b.String()
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
