package gopher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/chainagent"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/planner/react"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	sessnoop "trpc.group/trpc-go/trpc-agent-go/session/noop"

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

const compareSkillName = "report-compare"

var compareRunners sync.Map // userID -> *runnerEntry

// Compare 对选中论文逐篇检索正文，经 researcher、writer、reviewer 三段流水线生成对比报告。
func Compare(ctx context.Context, papers []core.PaperCompareInput) (*core.Reply, error) {
	if len(papers) < 2 {
		return nil, fmt.Errorf("gopher: 至少需要两篇论文才能对比")
	}
	if len(papers) > constant.ComparePapersMaxCount {
		return nil, fmt.Errorf("gopher: 单次最多对比 %d 篇论文", constant.ComparePapersMaxCount)
	}

	userID := tenant.MustStudentID(ctx)
	rt, err := compareRunnerForUser(userID)
	if err != nil {
		return nil, err
	}
	ctx = core.WithComparePapers(ctx, compareScopes(papers))
	ctx = retrieval.WithRefSink(ctx)

	query, err := compareQuery(papers)
	if err != nil {
		return nil, err
	}
	ch, err := rt.Run(ctx, userID, sessionID, trpcmodel.NewUserMessage(query))
	if err != nil {
		return nil, err
	}
	content, err := planstream.CollectEvents(ctx, ch)
	if err != nil {
		if errors.Is(err, planstream.ErrMaxToolIterations) {
			return fallbackCompare(ctx, papers, err)
		}
		return nil, err
	}
	content = sanitizeReport(content)
	if content == "" {
		return nil, fmt.Errorf("gopher: 多论文对比报告内容为空")
	}

	refs := retrieval.DrainRefs(ctx)
	meta := compareReplyMeta(papers, refs, "agentic")
	if issues := validateCompareReport(content, len(papers)); len(issues) > 0 {
		meta["format_warnings"] = issues
	}
	return &core.Reply{
		Intent:  constant.IntentType("compare"),
		Content: content,
		Meta:    meta,
	}, nil
}

func compareRunnerForUser(userID string) (runner.Runner, error) {
	if userID == "" {
		return nil, fmt.Errorf("gopher compare: userID 不能为空")
	}
	models, err := aimodel.ModelsForUser(userID)
	if err != nil {
		return nil, err
	}
	entry, _ := compareRunners.LoadOrStore(userID, &runnerEntry{})
	cached := entry.(*runnerEntry)
	cached.once.Do(func() {
		researchGC := core.GenConfig(models.ChatMC)
		researchGC.Stream = true
		researchGC.ReasoningEffort = nil
		researchGC.Temperature = trpcmodel.Float64Ptr(constant.ReportTemperature)
		researchGC.FrequencyPenalty = trpcmodel.Float64Ptr(constant.ReportFrequencyPenalty)

		researcherOpts := []llmagent.Option{
			llmagent.WithModel(models.Chat),
			llmagent.WithGenerationConfig(researchGC),
			llmagent.WithPlanner(react.New()),
			llmagent.WithMaxToolIterations(constant.AgenticMaxIterCompare),
			llmagent.WithTools(ragtools.CompareAll()),
			llmagent.WithInstruction(constant.CompareResearcherPrompt),
		}

		writerGC := core.GenConfig(models.ChatMC)
		writerGC.Stream = true
		writerGC.Temperature = trpcmodel.Float64Ptr(constant.ReportTemperature)
		writerGC.FrequencyPenalty = trpcmodel.Float64Ptr(constant.ReportFrequencyPenalty)
		writerOpts := []llmagent.Option{
			llmagent.WithModel(models.Chat),
			llmagent.WithGenerationConfig(writerGC),
			llmagent.WithInstruction(constant.CompareWriterPrompt),
		}

		reviewerGC := writerGC
		reviewerGC.Temperature = trpcmodel.Float64Ptr(constant.ReportReviewTemperature)
		reviewerOpts := []llmagent.Option{
			llmagent.WithModel(models.Chat),
			llmagent.WithGenerationConfig(reviewerGC),
			llmagent.WithInstruction(constant.CompareReviewerPrompt),
		}

		if repo := toolkit.SkillRepoFor(constant.AgentGopher); repo != nil {
			researcherOpts = append(researcherOpts, llmagent.WithSkills(repo))
			writerOpts = append(writerOpts, llmagent.WithSkills(repo))
			reviewerOpts = append(reviewerOpts, llmagent.WithSkills(repo))
		}

		pipeline := chainagent.New("gopher-compare", chainagent.WithSubAgents([]agent.Agent{
			newReportStageAgent(
				llmagent.New("gopher-compare-researcher", researcherOpts...),
				constant.ReportPhaseResearching,
				"逐篇检索：小囊鼠正在核对研究问题、方法、实验、数据集与结论证据。",
				"研究员正在为每篇论文建立可比较的证据清单。",
			),
			newReportStageAgent(
				llmagent.New("gopher-compare-writer", writerOpts...),
				constant.ReportPhaseWriting,
				"对齐写作：正在建立跨论文比较矩阵并判断实验可比性。",
				"撰写员正在把逐篇证据归并成对比维度和结论边界。",
			),
			newReportStageAgent(
				llmagent.New("gopher-compare-reviewer", reviewerOpts...),
				constant.ReportPhaseReviewing,
				"交叉审校：正在核对证据归属、缺失项与结论边界。",
				"评审员正在检查论文归属、指标可比性和遗漏风险。",
			),
		}))
		cached.rt = runner.NewRunner(
			appName,
			pipeline,
			runner.WithSessionService(sessnoop.NewService()),
		)
	})
	return cached.rt, cached.err
}

func compareQuery(papers []core.PaperCompareInput) (string, error) {
	payload, err := json.MarshalIndent(papers, "", "  ")
	if err != nil {
		return "", fmt.Errorf("gopher: 序列化论文对比输入失败: %w", err)
	}
	return strings.ReplaceAll(constant.CompareReportPrompt, "{papers}", string(payload)), nil
}

func compareScopes(papers []core.PaperCompareInput) []core.ComparePaperScope {
	scopes := make([]core.ComparePaperScope, 0, len(papers))
	for _, paper := range papers {
		scopes = append(scopes, core.ComparePaperScope{
			ID:    paper.ID,
			Title: comparePaperName(paper),
		})
	}
	return scopes
}

func comparePaperName(paper core.PaperCompareInput) string {
	if title := strings.TrimSpace(paper.Title); title != "" {
		return title
	}
	if fileName := strings.TrimSpace(paper.FileName); fileName != "" {
		return fileName
	}
	return paper.ID
}

func compareReplyMeta(
	papers []core.PaperCompareInput,
	refs []retrieval.Reference,
	mode string,
) map[string]any {
	ids := make([]string, 0, len(papers))
	for _, paper := range papers {
		ids = append(ids, paper.ID)
	}
	meta := map[string]any{
		"paper_ids":     ids,
		"skill":         compareSkillName,
		"pipeline":      []string{"researcher", "writer", "reviewer"},
		"generate_mode": mode,
		"source_count":  len(refs),
	}
	if len(refs) > 0 {
		meta["sources"] = refs
		counts := make(map[string]int, len(papers))
		for _, ref := range refs {
			if ref.DocID != "" {
				counts[ref.DocID]++
			}
		}
		meta["source_counts"] = counts
	}
	return meta
}

const (
	compareFallbackDocsPerQuery = 2
	compareFallbackMaxPerPaper  = 10
)

func fallbackCompare(
	ctx context.Context,
	papers []core.PaperCompareInput,
	cause error,
) (*core.Reply, error) {
	userID := tenant.MustStudentID(ctx)
	zlog.Warn("多论文对比工具迭代耗尽，启用固定检索兜底", "papers", len(papers), "err", cause)
	emitReportPhase(ctx, constant.ReportPhaseWriting, "检索轮数已达上限，正在使用逐篇固定证据生成保守对比。")

	evidence := fixedCompareEvidence(ctx, userID, papers)
	prompt := strings.ReplaceAll(constant.CompareFallbackPrompt, "{context}", evidence)
	models, err := aimodel.ModelsForUser(userID)
	if err != nil {
		return nil, err
	}
	gc := core.GenConfig(models.ChatMC)
	gc.Temperature = trpcmodel.Float64Ptr(constant.ReportTemperature)
	gc.FrequencyPenalty = trpcmodel.Float64Ptr(constant.ReportFrequencyPenalty)
	request := &trpcmodel.Request{
		Messages: []trpcmodel.Message{
			trpcmodel.NewSystemMessage(prompt),
			trpcmodel.NewUserMessage("请生成最终版多论文对比报告。"),
		},
		GenerationConfig: gc,
	}
	content, err := core.GenerateText(ctx, models.Chat, request)
	if err != nil {
		return nil, fmt.Errorf("gopher: 兜底对比报告生成失败: %w", err)
	}
	content = sanitizeReport(content)
	refs := retrieval.DrainRefs(ctx)
	meta := compareReplyMeta(papers, refs, "fallback")
	meta["fallback_reason"] = "max_tool_iterations"
	if issues := validateCompareReport(content, len(papers)); len(issues) > 0 {
		meta["format_warnings"] = issues
	}
	return &core.Reply{
		Intent:  constant.IntentType("compare"),
		Content: content,
		Meta:    meta,
	}, nil
}

func fixedCompareEvidence(
	ctx context.Context,
	ownerID string,
	papers []core.PaperCompareInput,
) string {
	var output strings.Builder
	for _, paper := range papers {
		fmt.Fprintf(&output, "\n## 论文：%s\npaper_id: %s\n", comparePaperName(paper), paper.ID)
		if structured, err := json.Marshal(paper); err == nil {
			fmt.Fprintf(&output, "结构化抽取：%s\n", structured)
		}

		seen := map[string]struct{}{}
		docs := make([]*retrieval.Doc, 0, compareFallbackMaxPerPaper)
		for _, query := range compareFallbackQueries() {
			found, err := retrieval.RetrieveForPaper(ctx, query, ownerID, paper.ID)
			if err != nil {
				zlog.Error("兜底对比正文检索失败", "paper_id", paper.ID, "query", query, "err", err)
				continue
			}
			docs = appendUniqueDocs(
				docs,
				seen,
				retrieval.DropImageDocs(found),
				compareFallbackDocsPerQuery,
				compareFallbackMaxPerPaper,
			)
		}
		retrieval.AddRefs(ctx, retrieval.References(docs))
		fmt.Fprintf(&output, "正文证据：\n%s\n", retrieval.FormatDocs(docs))
	}
	return output.String()
}

func compareFallbackQueries() []string {
	return []string{
		"研究问题 任务定义 动机 挑战",
		"方法 架构 关键模块 训练 推理",
		"实验设置 dataset benchmark 数据集 指标 基线",
		"主要结果 数值 消融 鲁棒性 结论",
		"创新点 局限性 失败案例 future work",
	}
}
