package paper

import (
	"strings"
	"testing"

	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
)

func TestNormalizeAnnotationInputDefaultsColor(t *testing.T) {
	in, err := normalizeAnnotationInput(AnnotationInput{
		PageNo:      1,
		Text:        "  important sentence  ",
		Translation: "  重要句子  ",
		BoundingRect: model.AnnotationRect{
			X1: 1, Y1: 2, X2: 10, Y2: 12, Width: 9, Height: 10, PageNumber: 1,
		},
		Rects: model.AnnotationRects{
			{X1: 1, Y1: 2, X2: 10, Y2: 12, Width: 9, Height: 10, PageNumber: 1},
		},
	})
	if err != nil {
		t.Fatalf("normalizeAnnotationInput() error = %v", err)
	}
	if in.Text != "important sentence" {
		t.Fatalf("text should be trimmed, got %q", in.Text)
	}
	if in.Color != defaultAnnotationColor {
		t.Fatalf("color = %q, want %q", in.Color, defaultAnnotationColor)
	}
	if in.Translation != "重要句子" {
		t.Fatalf("translation should be trimmed, got %q", in.Translation)
	}
}

func TestNormalizeAnnotationInputAcceptsExpandedColors(t *testing.T) {
	in, err := normalizeAnnotationInput(AnnotationInput{
		PageNo: 1,
		Text:   "important sentence",
		Color:  "magenta",
		BoundingRect: model.AnnotationRect{
			X1: 1, Y1: 2, X2: 10, Y2: 12, Width: 9, Height: 10, PageNumber: 1,
		},
		Rects: model.AnnotationRects{
			{X1: 1, Y1: 2, X2: 10, Y2: 12, Width: 9, Height: 10, PageNumber: 1},
		},
	})
	if err != nil {
		t.Fatalf("normalizeAnnotationInput() error = %v", err)
	}
	if in.Color != "magenta" {
		t.Fatalf("color = %q, want magenta", in.Color)
	}
}

func TestNormalizeAnnotationInputAcceptsFreetextWithoutRects(t *testing.T) {
	in, err := normalizeAnnotationInput(AnnotationInput{
		PageNo: 1,
		Kind:   constant.AnnotationKindFreetext,
		Text:   "新增文字",
		Color:  "black",
		BoundingRect: model.AnnotationRect{
			X1: 1, Y1: 2, X2: 10, Y2: 12, Width: 9, Height: 10, PageNumber: 1,
		},
		StyleJSON: model.JSONMap{"fontSize": 14},
	})
	if err != nil {
		t.Fatalf("normalizeAnnotationInput() error = %v", err)
	}
	if in.Kind != constant.AnnotationKindFreetext {
		t.Fatalf("kind = %q, want freetext", in.Kind)
	}
	if len(in.Rects) != 1 || in.Rects[0].PageNumber != 1 {
		t.Fatalf("rects should fallback to bounding rect, got %+v", in.Rects)
	}
}

func TestNormalizeAnnotationInputAcceptsDrawingContent(t *testing.T) {
	in, err := normalizeAnnotationInput(AnnotationInput{
		PageNo: 1,
		Kind:   constant.AnnotationKindDrawing,
		Color:  "gray",
		BoundingRect: model.AnnotationRect{
			X1: 1, Y1: 2, X2: 10, Y2: 12, Width: 9, Height: 10, PageNumber: 1,
		},
		ContentJSON: model.JSONMap{"image": "data:image/png;base64,abc"},
	})
	if err != nil {
		t.Fatalf("normalizeAnnotationInput() error = %v", err)
	}
	if in.Kind != constant.AnnotationKindDrawing {
		t.Fatalf("kind = %q, want drawing", in.Kind)
	}
	if in.Text != "" {
		t.Fatalf("drawing text should be optional, got %q", in.Text)
	}
}

func TestNormalizeAnnotationInputRejectsDrawingWithoutContent(t *testing.T) {
	_, err := normalizeAnnotationInput(AnnotationInput{
		PageNo: 1,
		Kind:   constant.AnnotationKindDrawing,
		BoundingRect: model.AnnotationRect{
			X1: 1, Y1: 2, X2: 10, Y2: 12, Width: 9, Height: 10, PageNumber: 1,
		},
	})
	if err == nil {
		t.Fatal("expected drawing without content to be rejected")
	}
}

func TestNormalizeAnnotationInputRejectsInvalidColor(t *testing.T) {
	_, err := normalizeAnnotationInput(AnnotationInput{
		PageNo: 1,
		Text:   "important sentence",
		Color:  "brown",
		BoundingRect: model.AnnotationRect{
			X1: 1, Y1: 2, X2: 10, Y2: 12, Width: 9, Height: 10, PageNumber: 1,
		},
		Rects: model.AnnotationRects{
			{X1: 1, Y1: 2, X2: 10, Y2: 12, Width: 9, Height: 10, PageNumber: 1},
		},
	})
	if err == nil {
		t.Fatal("expected invalid colors to be rejected")
	}
}

func TestNormalizeAnnotationInputRejectsCrossPageRects(t *testing.T) {
	_, err := normalizeAnnotationInput(AnnotationInput{
		PageNo: 2,
		Text:   "important sentence",
		BoundingRect: model.AnnotationRect{
			X1: 1, Y1: 2, X2: 10, Y2: 12, Width: 9, Height: 10, PageNumber: 2,
		},
		Rects: model.AnnotationRects{
			{X1: 1, Y1: 2, X2: 10, Y2: 12, Width: 9, Height: 10, PageNumber: 3},
		},
	})
	if err == nil {
		t.Fatal("expected cross-page rects to be rejected")
	}
}

func TestNormalizeAnnotationInputRejectsLongNote(t *testing.T) {
	_, err := normalizeAnnotationInput(AnnotationInput{
		PageNo: 1,
		Text:   "important sentence",
		Note:   strings.Repeat("注", maxAnnotationNoteRunes+1),
		BoundingRect: model.AnnotationRect{
			X1: 1, Y1: 2, X2: 10, Y2: 12, Width: 9, Height: 10, PageNumber: 1,
		},
		Rects: model.AnnotationRects{
			{X1: 1, Y1: 2, X2: 10, Y2: 12, Width: 9, Height: 10, PageNumber: 1},
		},
	})
	if err == nil {
		t.Fatal("expected long notes to be rejected")
	}
}
