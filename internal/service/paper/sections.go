package paper

import (
	"context"
	"errors"
	"os"

	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
	"GopherPaper/internal/parser"
)

// RebuildSections 从已归档的 MinerU 产物重建章节目录,不重跑完整解析流水线。
func RebuildSections(ctx context.Context, ownerID, paperID string) ([]model.PaperSection, error) {
	p, err := owned(ctx, ownerID, paperID)
	if err != nil {
		return nil, err
	}
	sections, err := sectionsFromArtifactDir(paperID, mineruDir(paperID), p.Title)
	if err != nil {
		return nil, err
	}
	if err := paperdao.SaveSections(ctx, paperID, sections); err != nil {
		return nil, err
	}
	return paperdao.ListSections(ctx, paperID)
}

func sectionsFromArtifactDir(paperID, dir, paperTitle string) ([]model.PaperSection, error) {
	doc, err := parser.ParseArtifactDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, parser.ErrArtifactContentListNotFound) {
			return []model.PaperSection{}, nil
		}
		return nil, err
	}
	return toSections(paperID, doc, paperTitle), nil
}
