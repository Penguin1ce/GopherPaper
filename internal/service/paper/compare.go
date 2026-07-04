package paper

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/dao"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
	"GopherPaper/internal/sse"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

const compareReportSaveTimeout = 30 * time.Second

// CompareRun 是当前多论文对比任务的可恢复进度快照。
type CompareRun struct {
	PaperIDs []string             `json:"paper_ids"`
	Steps    []ReportProgressStep `json:"steps"`
	Live     bool                 `json:"live"`
	Failed   bool                 `json:"failed"`
	ReportID uint64               `json:"report_id,omitempty"`
}

// Compare 基于用户选中的多篇论文结构化元信息生成横向对比分析并落库。
func Compare(ctx context.Context, ownerID string, paperIDs []string) (*model.PaperCompareReport, error) {
	ids := normalizePaperIDs(paperIDs)
	if len(ids) < 2 {
		return nil, fmt.Errorf("service/paper: 至少选择两篇论文")
	}
	if len(ids) > constant.ComparePapersMaxCount {
		return nil, fmt.Errorf("service/paper: 单次最多对比 %d 篇论文", constant.ComparePapersMaxCount)
	}

	inputs := make([]core.PaperCompareInput, 0, len(ids))
	for _, id := range ids {
		p, err := owned(ctx, ownerID, id)
		if err != nil {
			return nil, err
		}
		meta, err := paperdao.GetMeta(ctx, id)
		if err != nil && !errors.Is(err, errs.ErrPaperNotFound) {
			return nil, err
		}
		inputs = append(inputs, compareInput(p, meta))
	}

	if err := startCompareProgress(ctx, ownerID, ids); err != nil {
		zlog.Error("对比进度快照初始化失败", "owner", ownerID, "paper_ids", ids, "err", err)
	}
	sse.PushCompareProgress(ownerID, ids, constant.ReportPhasePreparing, "小囊鼠已接收对比任务，正在建立逐篇检索计划。")
	upstreamStream := core.StreamFrom(ctx)
	ctx = core.WithStream(ctx, func(event core.StreamEvent) {
		if upstreamStream != nil {
			upstreamStream(event)
		}
		if event.Kind == constant.StreamEventPlan {
			pushCompareProgress(ctx, ownerID, ids, event.Phase, event.Delta)
		}
	})

	reply, err := ai.ComparePapers(ctx, inputs)
	if err != nil {
		failCompareProgress(ctx, ownerID, ids, "对比报告生成失败")
		sse.PushCompareProgress(ownerID, ids, constant.ReportPhaseFailed, "对比报告生成失败")
		return nil, err
	}
	report := &model.PaperCompareReport{
		OwnerID:  ownerID,
		Title:    compareTitle(inputs),
		PaperIDs: model.JSONStrings(ids),
		Content:  reply.Content,
		Meta:     model.JSONMap(reply.Meta),
	}
	if report.Meta == nil {
		report.Meta = model.JSONMap{}
	}
	report.Meta["compare_papers"] = comparePaperMeta(inputs)
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), compareReportSaveTimeout)
	defer cancel()
	if err := paperdao.SaveCompareReport(saveCtx, report); err != nil {
		failCompareProgress(ctx, ownerID, ids, "对比报告保存失败")
		sse.PushCompareProgress(ownerID, ids, constant.ReportPhaseFailed, "对比报告保存失败")
		return nil, err
	}
	if err := finishCompareProgress(ctx, ownerID, ids, report.ID); err != nil {
		zlog.Error("对比进度快照完成态写入失败", "owner", ownerID, "paper_ids", ids, "err", err)
	}
	sse.PushCompareProgress(ownerID, ids, "completed", "对比报告已完成")
	return report, nil
}

// CompareOverview 返回某次多论文对比的进度快照,供前端刷新后恢复后台任务。
func CompareOverview(ctx context.Context, ownerID string, paperIDs []string) (*CompareRun, bool, error) {
	ids := normalizePaperIDs(paperIDs)
	if len(ids) < 2 {
		return nil, false, fmt.Errorf("service/paper: 至少选择两篇论文")
	}
	for _, id := range ids {
		if _, err := owned(ctx, ownerID, id); err != nil {
			return nil, false, err
		}
	}
	run, ok, err := loadCompareProgress(ctx, ownerID, ids)
	if err != nil || !ok {
		return nil, ok, err
	}
	return &run, true, nil
}

// ListCompareReports 返回当前用户的历史多论文对比报告。
func ListCompareReports(ctx context.Context, ownerID string) ([]model.PaperCompareReport, error) {
	reports, err := paperdao.ListCompareReports(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if reports == nil {
		reports = []model.PaperCompareReport{}
	}
	return reports, nil
}

// DeleteCompareReport 校验归属后删除一份历史多论文对比报告。
func DeleteCompareReport(ctx context.Context, ownerID string, reportID uint64) error {
	report, err := paperdao.GetCompareReport(ctx, reportID)
	if err != nil {
		return err
	}
	if report.OwnerID != ownerID {
		return errs.ErrPaperForbidden
	}
	return paperdao.DeleteCompareReport(ctx, reportID)
}

func newCompareRun(ids []string, text string) CompareRun {
	return CompareRun{
		PaperIDs: append([]string(nil), ids...),
		Steps: []ReportProgressStep{{
			Phase: constant.ReportPhasePreparing,
			Text:  text,
		}},
		Live:   true,
		Failed: false,
	}
}

func startCompareProgress(ctx context.Context, ownerID string, ids []string) error {
	run := newCompareRun(ids, "小囊鼠已接收对比任务，正在建立逐篇检索计划。")
	return saveCompareProgress(context.WithoutCancel(ctx), ownerID, ids, run)
}

func pushCompareProgress(ctx context.Context, ownerID string, ids []string, phase, detail string) {
	if err := appendCompareProgress(ctx, ownerID, ids, phase, detail); err != nil {
		zlog.Error("对比进度快照追加失败", "owner", ownerID, "paper_ids", ids, "phase", phase, "err", err)
	}
	sse.PushCompareProgress(ownerID, ids, phase, detail)
}

func appendCompareProgress(ctx context.Context, ownerID string, ids []string, phase, detail string) error {
	if phase == "" && detail == "" {
		return nil
	}
	run, ok, err := loadCompareProgress(ctx, ownerID, ids)
	if err != nil {
		return err
	}
	if !ok {
		run = CompareRun{PaperIDs: append([]string(nil), ids...), Live: true, Failed: false}
	}
	run.PaperIDs = append([]string(nil), ids...)
	run.Live = true
	run.Failed = false
	steps := run.Steps
	last := len(steps) - 1
	if last >= 0 && steps[last].Phase == phase {
		steps[last].Text = appendReportStepText(steps[last].Text, detail)
	} else if len(steps) < constant.MaxExecutionSteps {
		steps = append(steps, ReportProgressStep{Phase: phase, Text: trimReportStepText(detail)})
	} else if compacted, ok := compactReportStepsForAppend(steps); ok {
		steps = append(compacted, ReportProgressStep{Phase: phase, Text: trimReportStepText(detail)})
	} else {
		return saveCompareProgress(context.WithoutCancel(ctx), ownerID, ids, run)
	}
	run.Steps = steps
	return saveCompareProgress(context.WithoutCancel(ctx), ownerID, ids, run)
}

func finishCompareProgress(ctx context.Context, ownerID string, ids []string, reportID uint64) error {
	run, ok, err := loadCompareProgress(ctx, ownerID, ids)
	if err != nil {
		return err
	}
	if !ok {
		run = CompareRun{PaperIDs: append([]string(nil), ids...)}
	}
	run.PaperIDs = append([]string(nil), ids...)
	run.Live = false
	run.Failed = false
	run.ReportID = reportID
	run.Steps = append(run.Steps, ReportProgressStep{Phase: "completed", Text: "对比报告已完成"})
	return saveCompareProgress(context.WithoutCancel(ctx), ownerID, ids, run)
}

func failCompareProgress(ctx context.Context, ownerID string, ids []string, detail string) {
	run, ok, err := loadCompareProgress(ctx, ownerID, ids)
	if err != nil {
		zlog.Error("对比进度快照读取失败", "owner", ownerID, "paper_ids", ids, "err", err)
		return
	}
	if !ok {
		run = CompareRun{PaperIDs: append([]string(nil), ids...)}
	}
	run.PaperIDs = append([]string(nil), ids...)
	run.Live = false
	run.Failed = true
	if detail != "" {
		run.Steps = append(run.Steps, ReportProgressStep{Phase: constant.ReportPhaseFailed, Text: detail})
	}
	if err := saveCompareProgress(context.WithoutCancel(ctx), ownerID, ids, run); err != nil {
		zlog.Error("对比进度快照失败态写入失败", "owner", ownerID, "paper_ids", ids, "err", err)
	}
}

func loadCompareProgress(ctx context.Context, ownerID string, ids []string) (CompareRun, bool, error) {
	raw, err := dao.Get(ctx, compareProgressKey(ownerID, ids))
	if errors.Is(err, dao.ErrCacheMiss) {
		return CompareRun{}, false, nil
	}
	if err != nil {
		return CompareRun{}, false, fmt.Errorf("service/paper: 读取对比进度快照失败: %w", err)
	}
	var run CompareRun
	if err := json.Unmarshal([]byte(raw), &run); err != nil {
		return CompareRun{}, false, fmt.Errorf("service/paper: 解析对比进度快照失败: %w", err)
	}
	return run, true, nil
}

func saveCompareProgress(ctx context.Context, ownerID string, ids []string, run CompareRun) error {
	run.PaperIDs = append([]string(nil), ids...)
	blob, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("service/paper: 序列化对比进度快照失败: %w", err)
	}
	return dao.SetTTL(ctx, compareProgressKey(ownerID, ids), blob, constant.CompareProgressCacheTTL)
}

func compareProgressKey(ownerID string, ids []string) string {
	return constant.CompareProgressCacheKeyPrefix + ownerID + ":" + comparePaperIDsHash(ids)
}

func comparePaperIDsHash(ids []string) string {
	h := sha1.New()
	for _, id := range ids {
		_, _ = h.Write([]byte(id))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func normalizePaperIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func compareTitle(inputs []core.PaperCompareInput) string {
	names := make([]string, 0, len(inputs))
	for _, in := range inputs {
		name := strings.TrimSpace(in.Title)
		if name == "" {
			name = strings.TrimSpace(in.FileName)
		}
		if name == "" {
			name = in.ID
		}
		names = append(names, name)
	}
	title := "多论文对比：" + strings.Join(names, " / ")
	runes := []rune(title)
	if len(runes) <= 120 {
		return title
	}
	return string(runes[:117]) + "..."
}

func comparePaperMeta(inputs []core.PaperCompareInput) []map[string]any {
	out := make([]map[string]any, 0, len(inputs))
	for _, in := range inputs {
		title := strings.TrimSpace(in.Title)
		if title == "" {
			title = strings.TrimSpace(in.FileName)
		}
		out = append(out, map[string]any{
			"id":        in.ID,
			"title":     title,
			"file_name": in.FileName,
		})
	}
	return out
}

func compareInput(p *model.Paper, meta *model.PaperMeta) core.PaperCompareInput {
	in := core.PaperCompareInput{
		ID:       p.ID,
		Title:    p.Title,
		FileName: p.FileName,
	}
	if meta == nil {
		return in
	}
	in.Authors = []string(meta.Authors)
	in.Affiliations = []string(meta.Affiliations)
	in.PublishYear = meta.PublishYear
	in.Venue = meta.Venue
	in.Abstract = meta.Abstract
	in.Keywords = []string(meta.Keywords)
	in.ResearchQuestions = []string(meta.ResearchQuestions)
	in.Methods = meta.Methods
	in.Experiments = meta.Experiments
	in.Results = meta.Results
	in.Innovations = []string(meta.Innovations)
	in.Limitations = []string(meta.Limitations)
	in.FutureWork = []string(meta.FutureWork)
	return in
}
