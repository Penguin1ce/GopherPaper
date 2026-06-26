package paper

import (
	"context"
	"testing"

	"GopherPaper/internal/model"
	"GopherPaper/pkg/errs"
)

func withPipelinePaperGetter(t *testing.T, fn func(context.Context, string) (*model.Paper, error)) {
	t.Helper()
	old := getPaperForPipeline
	getPaperForPipeline = fn
	t.Cleanup(func() { getPaperForPipeline = old })
}

func TestPaperPresentForPipelineStopsWhenDeleted(t *testing.T) {
	withPipelinePaperGetter(t, func(context.Context, string) (*model.Paper, error) {
		return nil, errs.ErrPaperNotFound
	})
	task := parseTask{PaperID: "p1", OwnerID: "stu-1"}
	if paperPresentForPipeline(context.Background(), task, "upsert_chunks") {
		t.Fatal("deleted paper should stop pipeline writes")
	}
}

func TestPaperPresentForPipelineRejectsOwnerMismatch(t *testing.T) {
	withPipelinePaperGetter(t, func(context.Context, string) (*model.Paper, error) {
		return &model.Paper{ID: "p1", OwnerID: "stu-2"}, nil
	})
	task := parseTask{PaperID: "p1", OwnerID: "stu-1"}
	if paperPresentForPipeline(context.Background(), task, "upsert_graph") {
		t.Fatal("owner mismatch should stop pipeline writes")
	}
}

func TestPaperPresentForPipelineAllowsOwnedPaper(t *testing.T) {
	withPipelinePaperGetter(t, func(context.Context, string) (*model.Paper, error) {
		return &model.Paper{ID: "p1", OwnerID: "stu-1"}, nil
	})
	task := parseTask{PaperID: "p1", OwnerID: "stu-1"}
	if !paperPresentForPipeline(context.Background(), task, "mark_ready") {
		t.Fatal("owned existing paper should allow pipeline writes")
	}
}
