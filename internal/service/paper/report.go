package paper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/dao"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
	"GopherPaper/internal/service/metrics"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

// reportLockTTL 是单类报告生成锁的存活时间，须长于一次带多模态 RAG 的报告生成耗时，
// 进程崩溃后到期自动释放，可重新生成。
const reportLockTTL = 5 * time.Minute

// ReportProgressStep 是报告生成过程中的一个执行计划片段。
type ReportProgressStep struct {
	Phase string `json:"phase"`
	Text  string `json:"text"`
}

// ReportRun 是某类报告当前生成态的可恢复快照,供前端在 SSE 丢帧/重连后补齐计划栏。
type ReportRun struct {
	ReportType constant.ReportType  `json:"type"`
	Steps      []ReportProgressStep `json:"steps"`
	Live       bool                 `json:"live"`
	Failed     bool                 `json:"failed"`
}

// Report 取某篇论文某类研读报告,仅限本人。
// 命中持久化缓存直接复用;未命中不在 HTTP 链路同步生成,而是触发一次后台预生成并返回
// ErrReportGenerating,由前端轮询直到命中缓存,避免把整段生成耗时压在这次请求上。
func Report(ctx context.Context, ownerID, paperID string, t constant.ReportType) (*core.Reply, error) {
	if _, err := owned(ctx, ownerID, paperID); err != nil {
		return nil, err
	}
	if reply, ok, err := cachedReport(ctx, paperID, t); err != nil {
		return nil, err
	} else if ok {
		return reply, nil
	}
	// 触发后台生成,生成锁在 worker 内保证只写一次,重复投递只会被去重为空操作。
	queueReportProgress(ctx, paperID, t)
	enqueueReport(ctx, reportTask{PaperID: paperID, OwnerID: ownerID, ReportType: t})
	return nil, errs.ErrReportGenerating
}

// ReadyReports 列出某篇论文已生成的研读报告类型,仅限本人,供前端进入时回填就绪态。
// 只读,不触发任何生成。先读 Redis 缓存(生成期前端会 5 秒轮询,避免一直打 MySQL),
// 未命中再查 DB 并回填缓存;缓存由 SaveReport 主动失效,故不会漏掉新生成的报告。
func ReadyReports(ctx context.Context, ownerID, paperID string) ([]constant.ReportType, error) {
	if _, err := owned(ctx, ownerID, paperID); err != nil {
		return nil, err
	}
	return readyReportTypes(ctx, paperID)
}

// ReportOverview 返回某篇论文报告与思路图的就绪态。报告运行态来自 Redis 锁与进度快照,
// 用于前端在 SSE 断开、晚订阅或重复点击时恢复小囊鼠的执行计划与入口绿点。
func ReportOverview(ctx context.Context, ownerID, paperID string) ([]constant.ReportType, []ReportRun, bool, error) {
	if _, err := owned(ctx, ownerID, paperID); err != nil {
		return nil, nil, false, err
	}
	ready, err := readyReportTypes(ctx, paperID)
	if err != nil {
		return nil, nil, false, err
	}
	readySet := make(map[constant.ReportType]bool, len(ready))
	for _, t := range ready {
		readySet[t] = true
	}
	running, err := runningReports(ctx, paperID, readySet)
	if err != nil {
		return nil, nil, false, err
	}
	flowReady, err := paperdao.HasPaperFlow(ctx, ownerID, paperID)
	if err != nil {
		return nil, nil, false, err
	}
	return ready, running, flowReady, nil
}

func readyReportTypes(ctx context.Context, paperID string) ([]constant.ReportType, error) {
	cacheKey := constant.ReportReadyCacheKeyPrefix + paperID
	if cached, err := dao.Get(ctx, cacheKey); err == nil {
		var types []constant.ReportType
		if json.Unmarshal([]byte(cached), &types) == nil {
			return types, nil
		}
		// 缓存内容损坏不致命,落到 DB 重建。
	}
	types, err := paperdao.ListReportTypes(ctx, paperID)
	if err != nil {
		return nil, err
	}
	// 回填缓存(空数组也缓存,区分未缓存与确无报告;写失败不影响本次返回)。
	if blob, mErr := json.Marshal(types); mErr == nil {
		if sErr := dao.SetTTL(ctx, cacheKey, blob, constant.ReportReadyCacheTTL); sErr != nil {
			zlog.Error("就绪报告缓存写入失败", "paper_id", paperID, "err", sErr)
		}
	}
	return types, nil
}

func runningReports(ctx context.Context, paperID string, readySet map[constant.ReportType]bool) ([]ReportRun, error) {
	runs := make([]ReportRun, 0)
	for _, t := range constant.AllReportTypes() {
		if readySet[t] {
			continue
		}
		locked, err := dao.Exists(ctx, reportLockKey(paperID, t))
		if err != nil {
			return nil, fmt.Errorf("service/paper: 查询报告锁失败: %w", err)
		}
		run, ok, err := loadReportProgress(ctx, paperID, t)
		if err != nil {
			return nil, err
		}
		if !ok && !locked {
			continue
		}
		if !ok {
			run = newReportRun(t, "小囊鼠已接收生成任务,正在恢复执行进度。")
		}
		run.ReportType = t
		if locked {
			run.Live = true
			run.Failed = false
		}
		if len(run.Steps) == 0 {
			run.Steps = []ReportProgressStep{{
				Phase: constant.ReportPhasePreparing,
				Text:  "小囊鼠已接收生成任务,正在启动研读流水线。",
			}}
		}
		if run.Live || run.Failed || locked {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

// invalidateReadyCache 失效某篇论文的就绪报告缓存,写入新报告后调用,下次查询从 DB 重建。
func invalidateReadyCache(ctx context.Context, paperID string) {
	if _, err := dao.Del(context.WithoutCancel(ctx), constant.ReportReadyCacheKeyPrefix+paperID); err != nil {
		zlog.Error("就绪报告缓存失效失败", "paper_id", paperID, "err", err)
	}
}

// ensureReport 保证某类报告存在并返回:命中缓存即复用,否则抢 Redis 锁后生成并落库。
// 抢锁失败说明已有 worker 或并发点击在生成,返回 ErrReportGenerating,只允许一次写入。
// 调用前须保证 ctx 已注入 owner 租户,供 ai.GenerateReport 选到该用户模型。
func ensureReport(ctx context.Context, paperID string, t constant.ReportType) (*core.Reply, error) {
	if reply, ok, err := cachedReport(ctx, paperID, t); err != nil {
		return nil, err
	} else if ok {
		return reply, nil
	}

	lockKey := reportLockKey(paperID, t)
	got, err := dao.SetNX(ctx, lockKey, "1", reportLockTTL)
	if err != nil {
		return nil, fmt.Errorf("service/paper: 申请报告锁失败: %w", err)
	}
	if !got {
		// 别人正在生成,不重复生成与写入。
		return nil, errs.ErrReportGenerating
	}
	defer func() { _, _ = dao.Del(context.WithoutCancel(ctx), lockKey) }()

	// 双检:抢到锁后可能别人刚写完缓存,直接复用避免重算。
	if reply, ok, err := cachedReport(ctx, paperID, t); err != nil {
		return nil, err
	} else if ok {
		return reply, nil
	}

	start := time.Now()
	ownerID := tenant.MustStudentID(ctx)
	metricSuccess := false
	var metricErr error
	defer func() {
		metrics.Record(ctx, metrics.ServiceReport, ownerID, paperID, "", metricSuccess, time.Since(start), metricErr)
	}()

	if err := startReportProgress(ctx, paperID, t); err != nil {
		zlog.Error("报告进度快照初始化失败", "paper_id", paperID, "type", string(t), "err", err)
	}

	var reply *core.Reply
	var genErr error
	if t == constant.ReportRelated {
		reply, genErr = generateRelatedResearch(ctx, paperID)
	} else {
		reply, genErr = ai.GenerateReport(ctx, paperID, t)
	}
	if genErr != nil {
		metricErr = genErr
		return nil, genErr
	}
	attachReportProgressSteps(ctx, paperID, t, reply)
	rec := &model.PaperReport{PaperID: paperID, ReportType: t, Content: reply.Content}
	if reply.Meta != nil {
		rec.Meta = model.JSONMap(reply.Meta)
	}
	if err := paperdao.SaveReport(ctx, rec); err != nil {
		zlog.Error("研读报告落库失败", "paper_id", paperID, "type", string(t), "err", err)
		metricErr = err
		return nil, err
	}
	// 落库成功才失效就绪缓存,让轮询/重开下一次查询看到这条新报告。
	invalidateReadyCache(ctx, paperID)
	metricSuccess = true
	return reply, nil
}

// cachedReport 查报告持久化缓存,命中返回 Reply 与 true,未命中返回 false。
func cachedReport(ctx context.Context, paperID string, t constant.ReportType) (*core.Reply, bool, error) {
	cached, err := paperdao.GetReport(ctx, paperID, t)
	if err == errs.ErrReportNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &core.Reply{Content: cached.Content, Meta: cached.Meta}, true, nil
}

func attachReportProgressSteps(ctx context.Context, paperID string, t constant.ReportType, reply *core.Reply) {
	if reply == nil {
		return
	}
	run, ok, err := loadReportProgress(ctx, paperID, t)
	if err != nil {
		zlog.Error("读取报告进度快照失败", "paper_id", paperID, "type", string(t), "err", err)
		return
	}
	if !ok || len(run.Steps) == 0 {
		return
	}
	if reply.Meta == nil {
		reply.Meta = map[string]any{}
	}
	reply.Meta[constant.MetaKeyExecutionSteps] = run.Steps
}

func reportLockKey(paperID string, t constant.ReportType) string {
	return fmt.Sprintf("report:lock:%s:%s", paperID, t)
}

func reportProgressKey(paperID string, t constant.ReportType) string {
	return fmt.Sprintf("%s%s:%s", constant.ReportProgressCacheKeyPrefix, paperID, t)
}

func newReportRun(t constant.ReportType, text string) ReportRun {
	return ReportRun{
		ReportType: t,
		Steps: []ReportProgressStep{{
			Phase: constant.ReportPhasePreparing,
			Text:  text,
		}},
		Live:   true,
		Failed: false,
	}
}

func startReportProgress(ctx context.Context, paperID string, t constant.ReportType) error {
	run := newReportRun(t, "小囊鼠已接收生成任务,正在启动研读流水线。")
	return saveReportProgress(context.WithoutCancel(ctx), paperID, t, run)
}

func queueReportProgress(ctx context.Context, paperID string, t constant.ReportType) {
	locked, err := dao.Exists(ctx, reportLockKey(paperID, t))
	if err != nil {
		zlog.Error("报告生成锁查询失败", "paper_id", paperID, "type", string(t), "err", err)
		return
	}
	if locked {
		return
	}
	run := newReportRun(t, "小囊鼠已接收生成请求,正在排队启动研读流水线。")
	if err := saveReportProgress(context.WithoutCancel(ctx), paperID, t, run); err != nil {
		zlog.Error("报告排队进度快照写入失败", "paper_id", paperID, "type", string(t), "err", err)
	}
}

func appendReportProgress(ctx context.Context, paperID string, t constant.ReportType, phase, detail string) error {
	if phase == "" && detail == "" {
		return nil
	}
	run, ok, err := loadReportProgress(ctx, paperID, t)
	if err != nil {
		return err
	}
	if !ok {
		run = ReportRun{ReportType: t, Live: true, Failed: false}
	}
	run.ReportType = t
	run.Live = true
	run.Failed = false
	steps := run.Steps
	last := len(steps) - 1
	if last >= 0 && steps[last].Phase == phase {
		steps[last].Text = appendReportStepText(steps[last].Text, detail)
	} else if len(steps) < constant.MaxExecutionSteps {
		steps = append(steps, ReportProgressStep{Phase: phase, Text: trimReportStepText(detail)})
	} else {
		return saveReportProgress(context.WithoutCancel(ctx), paperID, t, run)
	}
	run.Steps = steps
	return saveReportProgress(context.WithoutCancel(ctx), paperID, t, run)
}

func finishReportProgress(ctx context.Context, paperID string, t constant.ReportType) error {
	run, ok, err := loadReportProgress(ctx, paperID, t)
	if err != nil || !ok {
		return err
	}
	run.ReportType = t
	run.Live = false
	return saveReportProgress(context.WithoutCancel(ctx), paperID, t, run)
}

func failReportProgress(ctx context.Context, paperID string, t constant.ReportType, detail string) error {
	run, ok, err := loadReportProgress(ctx, paperID, t)
	if err != nil {
		return err
	}
	if !ok {
		run = ReportRun{ReportType: t}
	}
	run.ReportType = t
	run.Live = false
	run.Failed = true
	if detail != "" {
		run.Steps = append(run.Steps, ReportProgressStep{Phase: constant.ReportPhaseFailed, Text: detail})
	}
	return saveReportProgress(context.WithoutCancel(ctx), paperID, t, run)
}

func loadReportProgress(ctx context.Context, paperID string, t constant.ReportType) (ReportRun, bool, error) {
	raw, err := dao.Get(ctx, reportProgressKey(paperID, t))
	if errors.Is(err, dao.ErrCacheMiss) {
		return ReportRun{}, false, nil
	}
	if err != nil {
		return ReportRun{}, false, fmt.Errorf("service/paper: 读取报告进度快照失败: %w", err)
	}
	var run ReportRun
	if err := json.Unmarshal([]byte(raw), &run); err != nil {
		return ReportRun{}, false, fmt.Errorf("service/paper: 解析报告进度快照失败: %w", err)
	}
	return run, true, nil
}

func saveReportProgress(ctx context.Context, paperID string, t constant.ReportType, run ReportRun) error {
	run.ReportType = t
	blob, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("service/paper: 序列化报告进度快照失败: %w", err)
	}
	return dao.SetTTL(ctx, reportProgressKey(paperID, t), blob, constant.ReportProgressCacheTTL)
}

func appendReportStepText(base, extra string) string {
	return trimReportStepText(base + extra)
}

func trimReportStepText(s string) string {
	r := []rune(s)
	if len(r) <= constant.MaxExecutionStepTextRunes {
		return s
	}
	if constant.MaxExecutionStepTextRunes <= 3 {
		return string(r[:constant.MaxExecutionStepTextRunes])
	}
	return string(r[:constant.MaxExecutionStepTextRunes-3]) + "..."
}
