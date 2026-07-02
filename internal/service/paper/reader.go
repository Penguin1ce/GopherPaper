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
	defaultAnnotationColor        = string(constant.AnnotationColorYellow)
	maxAnnotationNoteRunes        = 2000
	maxAnnotationTranslationRunes = constant.MaxTranslateRunes * 2
	maxAnnotationContentRunes     = 2 << 20
)

// AnnotationInput 是创建精读批注所需的业务字段。
type AnnotationInput struct {
	PageNo       int
	Kind         constant.AnnotationKind
	Text         string
	Note         string
	Translation  string
	Color        string
	BoundingRect model.AnnotationRect
	Rects        model.AnnotationRects
	StyleJSON    model.JSONMap
	ContentJSON  model.JSONMap
}

// AnnotationUpdateInput 是更新精读批注的可编辑字段。
type AnnotationUpdateInput struct {
	Text         *string
	Note         *string
	Translation  *string
	Color        *string
	BoundingRect *model.AnnotationRect
	Rects        *model.AnnotationRects
	StyleJSON    *model.JSONMap
	ContentJSON  *model.JSONMap
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
		Kind:         in.Kind,
		PageNo:       in.PageNo,
		Text:         in.Text,
		Note:         in.Note,
		Translation:  in.Translation,
		Color:        in.Color,
		BoundingRect: in.BoundingRect,
		Rects:        in.Rects,
		StyleJSON:    in.StyleJSON,
		ContentJSON:  in.ContentJSON,
	}
	if err := paperdao.CreateAnnotation(ctx, annotation); err != nil {
		return nil, err
	}
	return annotation, nil
}

// UpdateAnnotation 更新批注内容、位置或样式。
func UpdateAnnotation(ctx context.Context, ownerID, paperID string, annotationID uint64, in AnnotationUpdateInput) (*model.PaperAnnotation, error) {
	if _, err := owned(ctx, ownerID, paperID); err != nil {
		return nil, err
	}
	annotation, err := ownedAnnotation(ctx, ownerID, paperID, annotationID)
	if err != nil {
		return nil, err
	}
	fields := map[string]any{}
	kind := normalizeAnnotationKind(annotation.Kind)
	if in.Text != nil {
		value := strings.TrimSpace(*in.Text)
		if annotationTextRequired(kind) && value == "" {
			return nil, fmt.Errorf("service/paper: 批注文本为空")
		}
		if value != "" && runeLen(value) > constant.MaxTranslateRunes {
			return nil, fmt.Errorf("service/paper: 批注文本过长")
		}
		fields["text"] = value
		annotation.Text = value
	}
	if in.Note != nil {
		value := strings.TrimSpace(*in.Note)
		if runeLen(value) > maxAnnotationNoteRunes {
			return nil, fmt.Errorf("service/paper: 批注笔记过长")
		}
		fields["note"] = value
		annotation.Note = value
	}
	if in.Translation != nil {
		value, err := normalizeAnnotationTranslation(*in.Translation)
		if err != nil {
			return nil, err
		}
		fields["translation"] = value
		annotation.Translation = value
	}
	if in.Color != nil {
		value, err := normalizeAnnotationColor(*in.Color)
		if err != nil {
			return nil, err
		}
		fields["color"] = value
		annotation.Color = value
	}
	if in.BoundingRect != nil {
		if err := validateAnnotationRect(*in.BoundingRect, annotation.PageNo); err != nil {
			return nil, err
		}
		fields["bounding_rect"] = *in.BoundingRect
		annotation.BoundingRect = *in.BoundingRect
	}
	if in.Rects != nil {
		rects := normalizeAnnotationRects(*in.Rects, annotation.BoundingRect)
		for _, rect := range rects {
			if err := validateAnnotationRect(rect, annotation.PageNo); err != nil {
				return nil, fmt.Errorf("service/paper: 批注坐标无效")
			}
		}
		fields["rects"] = rects
		annotation.Rects = rects
	}
	if in.StyleJSON != nil {
		fields["style_json"] = *in.StyleJSON
		annotation.StyleJSON = *in.StyleJSON
	}
	if in.ContentJSON != nil {
		if err := validateAnnotationContent(kind, *in.ContentJSON); err != nil {
			return nil, err
		}
		fields["content_json"] = *in.ContentJSON
		annotation.ContentJSON = *in.ContentJSON
	}
	if err := validateAnnotationByKind(kind, annotation.Text, annotation.ContentJSON); err != nil {
		return nil, err
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
	in.Kind = normalizeAnnotationKind(in.Kind)
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
	if err := validateAnnotationByKind(in.Kind, in.Text, in.ContentJSON); err != nil {
		return in, err
	}
	if in.Text != "" && runeLen(in.Text) > constant.MaxTranslateRunes {
		return in, fmt.Errorf("service/paper: 批注文本过长")
	}
	if runeLen(in.Note) > maxAnnotationNoteRunes {
		return in, fmt.Errorf("service/paper: 批注笔记过长")
	}
	color, err := normalizeAnnotationColor(in.Color)
	if err != nil {
		return in, err
	}
	in.Color = color
	if err := validateAnnotationRect(in.BoundingRect, in.PageNo); err != nil {
		return in, err
	}
	in.Rects = normalizeAnnotationRects(in.Rects, in.BoundingRect)
	for _, rect := range in.Rects {
		if err := validateAnnotationRect(rect, in.PageNo); err != nil {
			return in, fmt.Errorf("service/paper: 批注坐标无效")
		}
	}
	if err := validateAnnotationContent(in.Kind, in.ContentJSON); err != nil {
		return in, err
	}
	return in, nil
}

func normalizeAnnotationColor(color string) (string, error) {
	color = strings.TrimSpace(strings.ToLower(color))
	if color == "" {
		return defaultAnnotationColor, nil
	}
	if !constant.AnnotationColor(color).Valid() {
		return "", fmt.Errorf("service/paper: 不支持的批注颜色")
	}
	return color, nil
}

func normalizeAnnotationKind(kind constant.AnnotationKind) constant.AnnotationKind {
	if kind == "" {
		return constant.AnnotationKindSelection
	}
	if kind.Valid() {
		return kind
	}
	return constant.AnnotationKind("")
}

func annotationTextRequired(kind constant.AnnotationKind) bool {
	return kind == constant.AnnotationKindSelection || kind == constant.AnnotationKindFreetext
}

func normalizeAnnotationRects(rects model.AnnotationRects, fallback model.AnnotationRect) model.AnnotationRects {
	if len(rects) > 0 {
		return rects
	}
	return model.AnnotationRects{fallback}
}

func validateAnnotationByKind(kind constant.AnnotationKind, text string, content model.JSONMap) error {
	if !kind.Valid() {
		return fmt.Errorf("service/paper: 不支持的批注类型")
	}
	if annotationTextRequired(kind) && strings.TrimSpace(text) == "" {
		return fmt.Errorf("service/paper: 批注文本为空")
	}
	if kind == constant.AnnotationKindDrawing {
		return validateDrawingContent(content)
	}
	return nil
}

func validateAnnotationContent(kind constant.AnnotationKind, content model.JSONMap) error {
	if content == nil {
		content = model.JSONMap{}
	}
	if runeLen(fmt.Sprint(content)) > maxAnnotationContentRunes {
		return fmt.Errorf("service/paper: 批注内容过大")
	}
	if kind == constant.AnnotationKindDrawing {
		return validateDrawingContent(content)
	}
	return nil
}

func validateDrawingContent(content model.JSONMap) error {
	if content == nil {
		return fmt.Errorf("service/paper: 绘图内容为空")
	}
	if image, ok := content["image"].(string); ok && strings.HasPrefix(image, "data:image/") {
		return nil
	}
	if strokes, ok := content["strokes"].([]any); ok && len(strokes) > 0 {
		return nil
	}
	return fmt.Errorf("service/paper: 绘图内容为空")
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
