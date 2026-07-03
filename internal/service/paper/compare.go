package paper

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

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

	reply, err := ai.ComparePapers(ctx, inputs)
	if err != nil {
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
	if err := paperdao.SaveCompareReport(ctx, report); err != nil {
		return nil, err
	}
	return report, nil
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
	in.PublishYear = meta.PublishYear
	in.Venue = meta.Venue
	in.Keywords = []string(meta.Keywords)
	in.ResearchQuestions = []string(meta.ResearchQuestions)
	in.Methods = meta.Methods
	in.Experiments = meta.Experiments
	in.Results = meta.Results
	return in
}
