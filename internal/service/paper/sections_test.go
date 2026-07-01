package paper

import (
	"os"
	"path/filepath"
	"testing"

	"GopherPaper/internal/ai/core"
)

func TestSectionsFromArtifactDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "paper_content_list.json"), []byte(`[
		{"type":"text","text":"Introduction","text_level":1,"page_idx":0,"bbox":[10,20,100,40]},
		{"type":"text","text":"Method","text_level":1,"page_idx":2,"bbox":[11,33,111,55]}
	]`), 0o644); err != nil {
		t.Fatal(err)
	}

	sections, err := sectionsFromArtifactDir("paper-1", dir, "")
	if err != nil {
		t.Fatalf("sectionsFromArtifactDir() error = %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(sections))
	}
	if sections[1].PaperID != "paper-1" || sections[1].Title != "Method" || sections[1].PageNo != 3 {
		t.Fatalf("section mapping mismatch %+v", sections[1])
	}
	if sections[1].Y1 == nil || *sections[1].Y1 != 33 {
		t.Fatalf("section coordinate mismatch %+v", sections[1])
	}
}

func TestToSectionsPreservesCoordinates(t *testing.T) {
	ptr := func(value float64) *float64 {
		return &value
	}
	sections := toSections("paper-1", &core.ParsedDoc{Sections: []core.Section{
		{Level: 1, Title: "Method", PageNo: 3, OrderIdx: 7, X1: ptr(10), Y1: ptr(89), X2: ptr(500), Y2: ptr(112)},
	}}, "")

	if len(sections) != 1 {
		t.Fatalf("sections = %d, want 1", len(sections))
	}
	got := sections[0]
	if got.X1 == nil || *got.X1 != 10 || got.Y1 == nil || *got.Y1 != 89 || got.X2 == nil || *got.X2 != 500 || got.Y2 == nil || *got.Y2 != 112 {
		t.Fatalf("section coordinates not preserved %+v", got)
	}
}

func TestSectionsFromArtifactDirMissing(t *testing.T) {
	sections, err := sectionsFromArtifactDir("paper-1", filepath.Join(t.TempDir(), "missing"), "")
	if err != nil {
		t.Fatalf("sectionsFromArtifactDir() error = %v", err)
	}
	if len(sections) != 0 {
		t.Fatalf("sections = %d, want 0", len(sections))
	}
}

func TestSectionsFromArtifactDirNormalizesMinerULevels(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "paper_content_list.json"), []byte(`[
		{"type":"text","text":"A Paper Title","text_level":1,"page_idx":0},
		{"type":"text","text":"5 Conclusion","text_level":2,"page_idx":3},
		{"type":"text","text":"Limitations","text_level":2,"page_idx":4},
		{"type":"text","text":"Ethics Statement","text_level":2,"page_idx":4},
		{"type":"text","text":"A Appendix Example","text_level":2,"page_idx":5}
	]`), 0o644); err != nil {
		t.Fatal(err)
	}

	sections, err := sectionsFromArtifactDir("paper-1", dir, "A Paper Title")
	if err != nil {
		t.Fatalf("sectionsFromArtifactDir() error = %v", err)
	}
	if len(sections) != 4 {
		t.Fatalf("sections = %d, want 4", len(sections))
	}
	for _, section := range sections {
		if section.Level != 1 {
			t.Fatalf("%q level = %d, want 1", section.Title, section.Level)
		}
	}
	if sections[0].Title != "5 Conclusion" || sections[1].Title != "Limitations" || sections[2].Title != "Ethics Statement" {
		t.Fatalf("sections order/title mismatch %+v", sections)
	}
}

func TestSectionsFromArtifactDirFiltersTableLikeTitles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "paper_content_list.json"), []byte(`[
		{"type":"text","text":"4 Results","text_level":1,"page_idx":17},
		{"type":"text","text":"Case 1: Numerical Data Formatting × Everyday Instruction","text_level":1,"page_idx":18},
		{"type":"text","text":"Base Model","text_level":1,"page_idx":18},
		{"type":"text","text":"FLAS (Ours)","text_level":1,"page_idx":18},
		{"type":"text","text":"In-Context Prompting","text_level":1,"page_idx":18},
		{"type":"text","text":"Case 2: Numerical References × Science Explanation","text_level":1,"page_idx":18},
		{"type":"text","text":"Base Model","text_level":1,"page_idx":19},
		{"type":"text","text":"FLAS (Ours)","text_level":1,"page_idx":19},
		{"type":"text","text":"In-Context Prompting","text_level":1,"page_idx":19},
		{"type":"text","text":"Case 3: Time Indicators × Business Proposal","text_level":1,"page_idx":19},
		{"type":"text","text":"5 Conclusion","text_level":1,"page_idx":22}
	]`), 0o644); err != nil {
		t.Fatal(err)
	}

	sections, err := sectionsFromArtifactDir("paper-1", dir, "")
	if err != nil {
		t.Fatalf("sectionsFromArtifactDir() error = %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("sections = %d, want 2: %+v", len(sections), sections)
	}
	if sections[0].Title != "4 Results" || sections[1].Title != "5 Conclusion" {
		t.Fatalf("sections order/title mismatch %+v", sections)
	}
}
