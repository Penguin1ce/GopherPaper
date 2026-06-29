package paper

import (
	"strings"
	"testing"

	"GopherPaper/internal/model"
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
		Color:  "purple",
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
	if in.Color != "purple" {
		t.Fatalf("color = %q, want purple", in.Color)
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
