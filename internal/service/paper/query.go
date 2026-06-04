package paper

import (
	"context"

	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/errs"
)

// List 列出某用户的全部论文。
func List(ctx context.Context, ownerID string) ([]model.Paper, error) {
	return paperdao.List(ctx, ownerID)
}

// Search 在某用户论文里按关键词检索。
func Search(ctx context.Context, ownerID, keyword string) ([]model.Paper, error) {
	return paperdao.Search(ctx, ownerID, keyword)
}

// GetStatus 取论文解析状态,仅限本人,断线重连兜底用。
func GetStatus(ctx context.Context, ownerID, paperID string) (*model.Paper, error) {
	return owned(ctx, ownerID, paperID)
}

// Detail 取论文及其结构化元信息与章节,仅限本人。
func Detail(ctx context.Context, ownerID, paperID string) (*model.Paper, *model.PaperMeta, []model.PaperSection, error) {
	p, err := owned(ctx, ownerID, paperID)
	if err != nil {
		return nil, nil, nil, err
	}
	meta, err := paperdao.GetMeta(ctx, paperID)
	if err != nil && err != errs.ErrPaperNotFound {
		return nil, nil, nil, err
	}
	sections, err := paperdao.ListSections(ctx, paperID)
	if err != nil {
		return nil, nil, nil, err
	}
	return p, meta, sections, nil
}

// owned 取论文并校验归属,非本人返回 errs.ErrPaperForbidden。
func owned(ctx context.Context, ownerID, paperID string) (*model.Paper, error) {
	p, err := paperdao.Get(ctx, paperID)
	if err != nil {
		return nil, err
	}
	if p.OwnerID != ownerID {
		return nil, errs.ErrPaperForbidden
	}
	return p, nil
}
