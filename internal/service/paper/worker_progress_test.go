package paper

import (
	"testing"

	"GopherPaper/internal/model"
	"GopherPaper/internal/parser"
)

func TestNormalizeParseProgressComputesPercent(t *testing.T) {
	p := normalizeParseProgress(parser.Progress{Parsed: 7, Total: 18})
	if p.Percent != 38 || p.Parsed != 7 || p.Total != 18 {
		t.Fatalf("unexpected progress %+v", p)
	}
}

func TestNormalizeParseProgressClampsPagesAndPercent(t *testing.T) {
	p := normalizeParseProgress(parser.Progress{Percent: 120, Parsed: 20, Total: 18})
	if p.Percent != 100 || p.Parsed != 18 || p.Total != 18 {
		t.Fatalf("unexpected progress %+v", p)
	}
}

func TestKeepParseProgressMonotonicUsesExistingHigherProgress(t *testing.T) {
	existing := &model.Paper{
		ParseProgress: 45,
		ParsedPages:   9,
		TotalPages:    18,
	}
	p := keepParseProgressMonotonic(parser.Progress{Percent: 30, Parsed: 7, Total: 18}, existing)
	if p.Percent != 45 || p.Parsed != 9 || p.Total != 18 {
		t.Fatalf("unexpected progress %+v", p)
	}
}

func TestKeepParseProgressMonotonicAllowsCompletion(t *testing.T) {
	existing := &model.Paper{
		ParseProgress: 90,
		ParsedPages:   17,
		TotalPages:    18,
	}
	p := keepParseProgressMonotonic(parser.Progress{Percent: 100, Parsed: 18, Total: 18}, existing)
	if p.Percent != 100 || p.Parsed != 18 || p.Total != 18 {
		t.Fatalf("unexpected progress %+v", p)
	}
}

func TestKeepParseProgressMonotonicKeepsPagesOnPercentOnlyCompletion(t *testing.T) {
	existing := &model.Paper{
		ParseProgress: 90,
		ParsedPages:   17,
		TotalPages:    18,
	}
	p := keepParseProgressMonotonic(parser.Progress{Percent: 100}, existing)
	if p.Percent != 100 || p.Parsed != 17 || p.Total != 18 {
		t.Fatalf("unexpected progress %+v", p)
	}
}
