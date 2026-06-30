package parser

import "testing"

func TestExtractProgressTopLevelFields(t *testing.T) {
	p, ok := extractProgress(map[string]any{
		"parsed_pages": float64(7),
		"total_pages":  float64(18),
	})
	if !ok {
		t.Fatal("expected progress")
	}
	if p.Percent != 38 || p.Parsed != 7 || p.Total != 18 {
		t.Fatalf("unexpected progress %+v", p)
	}
}

func TestExtractProgressNestedAndPercentString(t *testing.T) {
	p, ok := extractProgress(map[string]any{
		"page_progress": map[string]any{
			"done":  "7",
			"total": "18",
			"rate":  "39%",
		},
	})
	if !ok {
		t.Fatal("expected progress")
	}
	if p.Percent != 39 || p.Parsed != 7 || p.Total != 18 {
		t.Fatalf("unexpected progress %+v", p)
	}
}

func TestExtractProgressFractionAndRatio(t *testing.T) {
	p, ok := extractProgress(map[string]any{
		"page_progress": "7/18",
		"ratio":         0.39,
	})
	if !ok {
		t.Fatal("expected progress")
	}
	if p.Percent != 39 || p.Parsed != 7 || p.Total != 18 {
		t.Fatalf("unexpected progress %+v", p)
	}
}

func TestExtractProgressClamps(t *testing.T) {
	p, ok := extractProgress(map[string]any{
		"parsed_pages": 20,
		"total_pages":  18,
		"progress":     120,
	})
	if !ok {
		t.Fatal("expected progress")
	}
	if p.Percent != 100 || p.Parsed != 18 || p.Total != 18 {
		t.Fatalf("unexpected progress %+v", p)
	}
}
