package paper

import (
	"reflect"
	"testing"

	"GopherPaper/internal/model"
)

func TestCompareInputIncludesAnalysisFields(t *testing.T) {
	paper := &model.Paper{
		ID:       "paper-1",
		Title:    "A Paper",
		FileName: "paper.pdf",
	}
	meta := &model.PaperMeta{
		Authors:           model.JSONStrings{"Alice"},
		Affiliations:      model.JSONStrings{"Example University"},
		PublishYear:       2026,
		Venue:             "ExampleConf",
		Abstract:          "Abstract",
		Keywords:          model.JSONStrings{"retrieval"},
		ResearchQuestions: model.JSONStrings{"How to compare papers?"},
		Methods:           "Method",
		Experiments:       "Experiment",
		Results:           "Result",
		Innovations:       model.JSONStrings{"Innovation"},
		Limitations:       model.JSONStrings{"Limitation"},
		FutureWork:        model.JSONStrings{"Future work"},
	}

	got := compareInput(paper, meta)

	if got.ID != paper.ID || got.Title != paper.Title || got.FileName != paper.FileName {
		t.Fatalf("compareInput() identity = (%q, %q, %q), want paper identity", got.ID, got.Title, got.FileName)
	}
	if !reflect.DeepEqual(got.Authors, []string{"Alice"}) {
		t.Errorf("Authors = %v", got.Authors)
	}
	if !reflect.DeepEqual(got.Affiliations, []string{"Example University"}) {
		t.Errorf("Affiliations = %v", got.Affiliations)
	}
	if got.PublishYear != 2026 || got.Venue != "ExampleConf" || got.Abstract != "Abstract" {
		t.Errorf("publication fields = (%d, %q, %q)", got.PublishYear, got.Venue, got.Abstract)
	}
	if !reflect.DeepEqual(got.Keywords, []string{"retrieval"}) ||
		!reflect.DeepEqual(got.ResearchQuestions, []string{"How to compare papers?"}) {
		t.Errorf("research fields = (%v, %v)", got.Keywords, got.ResearchQuestions)
	}
	if got.Methods != "Method" || got.Experiments != "Experiment" || got.Results != "Result" {
		t.Errorf("analysis text = (%q, %q, %q)", got.Methods, got.Experiments, got.Results)
	}
	if !reflect.DeepEqual(got.Innovations, []string{"Innovation"}) ||
		!reflect.DeepEqual(got.Limitations, []string{"Limitation"}) ||
		!reflect.DeepEqual(got.FutureWork, []string{"Future work"}) {
		t.Errorf("boundary fields = (%v, %v, %v)", got.Innovations, got.Limitations, got.FutureWork)
	}
}

func TestCompareInputWithoutMetaKeepsIdentity(t *testing.T) {
	paper := &model.Paper{ID: "paper-2", Title: "Title", FileName: "file.pdf"}
	got := compareInput(paper, nil)
	if got.ID != paper.ID || got.Title != paper.Title || got.FileName != paper.FileName {
		t.Fatalf("compareInput(nil meta) = %#v", got)
	}
}
