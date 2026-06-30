package paper

import (
	"context"
	"fmt"
	"math"
	"strings"

	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

const (
	defaultAnnotationColor        = "yellow"
	maxAnnotationNoteRunes        = 2000
	maxAnnotationTranslationRunes = constant.MaxTranslateRunes * 2
)

var annotationColors = map[string]bool{
	"yellow": true,
	"blue":   true,
	"green":  true,
	"pink":   true,
	"purple": true,
	"orange": true,
}

// AnnotationInput 是创建精读批注所需的业务字段。
type AnnotationInput struct {
	PageNo       int
	Text         string
	Note         string
	Translation  string
	Color        string
	BoundingRect model.AnnotationRect
	Rects        model.AnnotationRects
}

// UpdateReadProgress 保存某篇论文的最近阅读页与百分比进度。
func UpdateReadProgress(ctx context.Context, ownerID, paperID string, lastPage, totalPages int) (*model.Paper, error) {
	p, err := owned(ctx, ownerID, paperID)
	if err != nil {
		return nil, err
	}
	if lastPage < 0 {
		return nil, fmt.Errorf("service/paper: 阅读页码不能为负数")
	}
	pageCount := totalPages
	if pageCount <= 0 {
		pageCount = p.PageCount
	}
	if pageCount > 0 && lastPage > pageCount {
		lastPage = pageCount
	}
	progress := 0
	if pageCount > 0 && lastPage > 0 {
		progress = int(math.Round(float64(lastPage) / float64(pageCount) * 100))
	}
	progress = clampInt(progress, 0, 100)
	if err := paperdao.UpdateProgress(ctx, paperID, progress, lastPage); err != nil {
		return nil, err
	}
	p.Progress = progress
	p.LastReadPage = lastPage
	return p, nil
}

// ListAnnotations 列出某篇论文的精读批注。
func ListAnnotations(ctx context.Context, ownerID, paperID string) ([]model.PaperAnnotation, error) {
	if _, err := owned(ctx, ownerID, paperID); err != nil {
		return nil, err
	}
	return paperdao.ListAnnotations(ctx, paperID)
}

// CreateAnnotation 保存一条新的精读批注。
func CreateAnnotation(ctx context.Context, ownerID, paperID string, in AnnotationInput) (*model.PaperAnnotation, error) {
	if _, err := owned(ctx, ownerID, paperID); err != nil {
		return nil, err
	}
	in, err := normalizeAnnotationInput(in)
	if err != nil {
		return nil, err
	}
	annotation := &model.PaperAnnotation{
		PaperID:      paperID,
		OwnerID:      ownerID,
		PageNo:       in.PageNo,
		Text:         in.Text,
		Note:         in.Note,
		Translation:  in.Translation,
		Color:        in.Color,
		BoundingRect: in.BoundingRect,
		Rects:        in.Rects,
	}
	if err := paperdao.CreateAnnotation(ctx, annotation); err != nil {
		return nil, err
	}
	return annotation, nil
}

// UpdateAnnotation 更新批注颜色或笔记。
func UpdateAnnotation(ctx context.Context, ownerID, paperID string, annotationID uint64, note, translation, color *string) (*model.PaperAnnotation, error) {
	if _, err := owned(ctx, ownerID, paperID); err != nil {
		return nil, err
	}
	annotation, err := ownedAnnotation(ctx, ownerID, paperID, annotationID)
	if err != nil {
		return nil, err
	}
	fields := map[string]any{}
	if note != nil {
		value := strings.TrimSpace(*note)
		if runeLen(value) > maxAnnotationNoteRunes {
			return nil, fmt.Errorf("service/paper: 批注笔记过长")
		}
		fields["note"] = value
		annotation.Note = value
	}
	if translation != nil {
		value, err := normalizeAnnotationTranslation(*translation)
		if err != nil {
			return nil, err
		}
		fields["translation"] = value
		annotation.Translation = value
	}
	if color != nil {
		value, err := normalizeAnnotationColor(*color)
		if err != nil {
			return nil, err
		}
		fields["color"] = value
		annotation.Color = value
	}
	if err := paperdao.UpdateAnnotation(ctx, annotationID, fields); err != nil {
		return nil, err
	}
	return annotation, nil
}

// DeleteAnnotation 删除一条精读批注。
func DeleteAnnotation(ctx context.Context, ownerID, paperID string, annotationID uint64) error {
	if _, err := owned(ctx, ownerID, paperID); err != nil {
		return err
	}
	if _, err := ownedAnnotation(ctx, ownerID, paperID, annotationID); err != nil {
		return err
	}
	return paperdao.DeleteAnnotation(ctx, annotationID)
}

func ownedAnnotation(ctx context.Context, ownerID, paperID string, annotationID uint64) (*model.PaperAnnotation, error) {
	annotation, err := paperdao.GetAnnotation(ctx, annotationID)
	if err != nil {
		return nil, err
	}
	if annotation.PaperID != paperID {
		return nil, errs.ErrAnnotationNotFound
	}
	if annotation.OwnerID != ownerID {
		return nil, errs.ErrPaperForbidden
	}
	return annotation, nil
}

func normalizeAnnotationInput(in AnnotationInput) (AnnotationInput, error) {
	in.Text = strings.TrimSpace(in.Text)
	in.Note = strings.TrimSpace(in.Note)
	var err error
	in.Translation, err = normalizeAnnotationTranslation(in.Translation)
	if err != nil {
		return in, err
	}
	if in.PageNo <= 0 {
		return in, fmt.Errorf("service/paper: 批注页码无效")
	}
	if in.Text == "" {
		return in, fmt.Errorf("service/paper: 批注原文为空")
	}
	if runeLen(in.Text) > constant.MaxTranslateRunes {
		return in, fmt.Errorf("service/paper: 批注原文过长")
	}
	if runeLen(in.Note) > maxAnnotationNoteRunes {
		return in, fmt.Errorf("service/paper: 批注笔记过长")
	}
	color, err := normalizeAnnotationColor(in.Color)
	if err != nil {
		return in, err
	}
	in.Color = color
	if len(in.Rects) == 0 {
		return in, fmt.Errorf("service/paper: 批注位置为空")
	}
	if err := validateAnnotationRect(in.BoundingRect, in.PageNo); err != nil {
		return in, err
	}
	for _, rect := range in.Rects {
		if err := validateAnnotationRect(rect, in.PageNo); err != nil {
			return in, fmt.Errorf("service/paper: 批注坐标无效")
		}
	}
	return in, nil
}

func normalizeAnnotationColor(color string) (string, error) {
	color = strings.TrimSpace(strings.ToLower(color))
	if color == "" {
		return defaultAnnotationColor, nil
	}
	if !annotationColors[color] {
		return "", fmt.Errorf("service/paper: 不支持的批注颜色")
	}
	return color, nil
}

func normalizeAnnotationTranslation(translation string) (string, error) {
	translation = strings.TrimSpace(translation)
	if runeLen(translation) > maxAnnotationTranslationRunes {
		return "", fmt.Errorf("service/paper: 鎵规敞璇戞枃杩囬暱")
	}
	return translation, nil
}

func invalidRectNumber(n float64) bool {
	return math.IsNaN(n) || math.IsInf(n, 0) || n < 0
}

func validateAnnotationRect(rect model.AnnotationRect, pageNo int) error {
	if rect.PageNumber != pageNo {
		return fmt.Errorf("service/paper: 仅支持单页批注")
	}
	if invalidRectNumber(rect.X1) || invalidRectNumber(rect.Y1) ||
		invalidRectNumber(rect.X2) || invalidRectNumber(rect.Y2) ||
		invalidRectNumber(rect.Width) || invalidRectNumber(rect.Height) {
		return fmt.Errorf("service/paper: 批注坐标无效")
	}
	if rect.X2 < rect.X1 || rect.Y2 < rect.Y1 || rect.Width <= 0 || rect.Height <= 0 {
		return fmt.Errorf("service/paper: 批注坐标无效")
	}
	return nil
}

func clampInt(n, min, max int) int {
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}
