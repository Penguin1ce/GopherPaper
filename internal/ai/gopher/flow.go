// gopher flow.go 是小囊鼠的「论文思路图」子能力:单轮 agent 挂 infographic-charts skill,
// 先用 ragtools 检索本篇论文的研究脉络证据,再按 skill 的 SVG 规范画一张研究思路流程图。
// 与小云雀的 React Flow 思路图(toolkit/paper_flow.go,JSON DAG)互补:这版产自包含 SVG,可下载可进 PDF。
// 按需生成不缓存、不落库,与 report 的多 agent 流水线分开(那条的 sanitizeReport 会按 H1 截断、破坏 SVG)。
package gopher

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	sessnoop "trpc.group/trpc-go/trpc-agent-go/session/noop"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/ragtools"
	"GopherPaper/internal/ai/retrieval"
	"GopherPaper/internal/ai/toolkit"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

const flowAgentName = "gopher-flow"

// flowRunners 按 userID 缓存思路图 runner,与 aimodel 的模型缓存一一对应。
var flowRunners sync.Map // userID -> *runnerEntry

// GenerateFlowSVG 围绕某篇论文画一张研究思路流程图,返回一段自包含 SVG。
// owner 从 ctx 的 tenant 取;ctx 注入 paperID 让检索工具限定到本篇。同步聚合,不流式、不缓存。
func GenerateFlowSVG(ctx context.Context, paperID string) (*core.Reply, error) {
	userID := tenant.MustStudentID(ctx)
	rt, err := flowRunnerForUser(userID)
	if err != nil {
		return nil, err
	}
	ctx = core.WithPaperID(ctx, paperID)

	ch, err := rt.Run(ctx, userID, sessionID, trpcmodel.NewUserMessage("请为这篇论文画一张研究思路流程图。"))
	if err != nil {
		return fallbackFlowSVG(ctx, paperID, err)
	}
	content, err := core.CollectEvents(ctx, ch)
	if err != nil {
		return fallbackFlowSVG(ctx, paperID, err)
	}
	svg := extractSVG(content)
	if svg == "" {
		return fallbackFlowSVG(ctx, paperID, fmt.Errorf("gopher: 思路图未产出有效 SVG"))
	}
	return &core.Reply{
		Content: svg,
		Meta:    map[string]any{"format": "svg"},
	}, nil
}

// flowRunnerForUser 懒建该用户的思路图 runner:单个 llmagent,挂 ragtools 检索工具与
// gopher-flow 分组 skill(infographic-charts),不挂 react planner、不分多段。
func flowRunnerForUser(userID string) (runner.Runner, error) {
	if userID == "" {
		return nil, fmt.Errorf("gopher: userID 不能为空")
	}
	models, err := aimodel.ModelsForUser(userID)
	if err != nil {
		return nil, err
	}
	e, _ := flowRunners.LoadOrStore(userID, &runnerEntry{})
	ent := e.(*runnerEntry)
	ent.once.Do(func() {
		gc := core.GenConfig(models.ChatMC)
		// 挂工具,部分 openai 兼容端点不能同用 reasoning_effort(400),剥离走端点默认;沿用 gopher report。
		gc.ReasoningEffort = nil
		// 后端仍同步聚合,但流式模式让工具循环的最终文本以更通用的事件形态返回。
		gc.Stream = true
		gc.Temperature = trpcmodel.Float64Ptr(constant.FlowTemperature)
		gc.FrequencyPenalty = trpcmodel.Float64Ptr(constant.FlowFrequencyPenalty)
		opts := []llmagent.Option{
			llmagent.WithModel(models.Chat),
			llmagent.WithGenerationConfig(gc),
			llmagent.WithMaxToolIterations(constant.AgenticMaxIterReport),
			llmagent.WithTools(ragtools.All()),
			llmagent.WithInstruction(constant.GopherFlowPrompt),
		}
		if repo := toolkit.SkillRepoFor(constant.AgentGopherFlow); repo != nil {
			opts = append(opts, llmagent.WithSkills(repo))
		}
		ent.rt = runner.NewRunner(appName, llmagent.New(flowAgentName, opts...),
			runner.WithSessionService(sessnoop.NewService()))
	})
	return ent.rt, ent.err
}

const (
	flowFallbackDocsPerQuery = 2
	flowFallbackMaxTextDocs  = 14
)

func fallbackFlowSVG(ctx context.Context, paperID string, cause error) (*core.Reply, error) {
	if ctx.Err() != nil {
		return nil, cause
	}
	userID := tenant.MustStudentID(ctx)
	zlog.Warn("思路图 agent 未产出有效 SVG,启用固定检索兜底", "paper_id", paperID, "err", cause)

	docs := fallbackFlowDocs(ctx, userID, paperID)
	prompt := strings.ReplaceAll(constant.GopherFallbackFlowPrompt, "{context}", retrieval.FormatDocs(docs))

	models, err := aimodel.ModelsForUser(userID)
	if err != nil {
		return nil, err
	}
	gc := core.GenConfig(models.ChatMC)
	gc.Temperature = trpcmodel.Float64Ptr(constant.FlowTemperature)
	gc.FrequencyPenalty = trpcmodel.Float64Ptr(constant.FlowFrequencyPenalty)
	req := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(prompt),
			trpcmodel.NewUserMessage("请生成最终 SVG 思路图。"),
		},
		GenerationConfig: gc,
	}
	content, err := core.GenerateText(ctx, models.Chat, req)
	if err != nil {
		return nil, fmt.Errorf("gopher: 兜底思路图生成失败: %w", err)
	}
	svg := extractSVG(content)
	if svg == "" {
		return nil, fmt.Errorf("gopher: 兜底思路图未产出有效 SVG")
	}
	meta := map[string]any{
		"format":          "svg",
		"fallback":        true,
		"fallback_reason": cause.Error(),
	}
	if sources := retrieval.References(docs); len(sources) > 0 {
		meta["sources"] = sources
	}
	return &core.Reply{Content: svg, Meta: meta}, nil
}

func fallbackFlowDocs(ctx context.Context, ownerID, paperID string) []*retrieval.Doc {
	seen := map[string]struct{}{}
	docs := make([]*retrieval.Doc, 0, flowFallbackMaxTextDocs)
	for _, q := range fallbackFlowQueries() {
		got, err := retrieval.RetrieveForPaper(ctx, q, ownerID, paperID)
		if err != nil {
			zlog.Error("兜底思路图检索失败", "paper_id", paperID, "query", q, "err", err)
			continue
		}
		docs = appendUniqueDocs(docs, seen, retrieval.DropImageDocs(got), flowFallbackDocsPerQuery, flowFallbackMaxTextDocs)
		if len(docs) >= flowFallbackMaxTextDocs {
			break
		}
	}
	return docs
}

func fallbackFlowQueries() []string {
	return []string{
		"研究问题 背景 动机 痛点",
		"现有方法不足 挑战 gap limitation",
		"核心思路 创新点 contribution idea",
		"方法流程 模型架构 算法步骤 pipeline",
		"实验设置 数据集 指标 baseline",
		"关键结果 main result ablation finding",
		"结论 贡献 局限 future work",
	}
}

// extractSVG 从模型输出里截取首个 <svg 到末个 </svg>,剥掉可能的 ```svg 围栏与说明文字。
func extractSVG(s string) string {
	s = strings.TrimSpace(s)
	start := strings.Index(s, "<svg")
	end := strings.LastIndex(s, "</svg>")
	if start < 0 || end < 0 || end < start {
		return ""
	}
	return strings.TrimSpace(s[start : end+len("</svg>")])
}
