package toolkit

import (
	"context"
	"errors"
	"testing"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/model"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

func withDeletePaperStubs(
	t *testing.T,
	papers map[string]*model.Paper,
	deleteFn func(context.Context, string, string) error,
) {
	t.Helper()
	oldGet := getPaperForTool
	oldDelete := deletePaperForTool
	getPaperForTool = func(ctx context.Context, id string) (*model.Paper, error) {
		if p, ok := papers[id]; ok {
			return p, nil
		}
		return nil, errs.ErrPaperNotFound
	}
	deletePaperForTool = deleteFn
	t.Cleanup(func() {
		getPaperForTool = oldGet
		deletePaperForTool = oldDelete
	})
}

func paperToolCtx(owner string) context.Context {
	return tenant.With(context.Background(), tenant.Tenant{StudentID: owner})
}

func clearPendingPaperDeleteConfirmationsForTest(t *testing.T) {
	t.Helper()
	pendingPaperDeleteConfirmations.Range(func(key, value any) bool {
		pendingPaperDeleteConfirmations.Delete(key)
		return true
	})
	t.Cleanup(func() {
		pendingPaperDeleteConfirmations.Range(func(key, value any) bool {
			pendingPaperDeleteConfirmations.Delete(key)
			return true
		})
	})
}

func TestDeleteMyPaperRejectsMissingPaperID(t *testing.T) {
	var calls int
	withDeletePaperStubs(t, map[string]*model.Paper{
		"p1": {ID: "p1", OwnerID: "stu-1", Title: "Attention Is All You Need", FileName: "attention.pdf"},
	}, func(context.Context, string, string) error {
		calls++
		return nil
	})
	ctx := paperToolCtx("stu-1")

	if _, err := deleteMyPaper(ctx, deletePaperInput{}); err == nil {
		t.Fatal("empty paper_id should be rejected")
	}
	if calls != 0 {
		t.Fatalf("delete function should not be called, got %d calls", calls)
	}
}

func TestDeleteMyPaperRejectsForeignPaper(t *testing.T) {
	var calls int
	withDeletePaperStubs(t, map[string]*model.Paper{
		"p1": {ID: "p1", OwnerID: "stu-2", Title: "Owned By Someone Else", FileName: "foreign.pdf"},
	}, func(context.Context, string, string) error {
		calls++
		return nil
	})

	_, err := deleteMyPaper(paperToolCtx("stu-1"), deletePaperInput{
		PaperID: "p1",
	})
	if !errors.Is(err, errs.ErrPaperForbidden) {
		t.Fatalf("expected ErrPaperForbidden, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("delete function should not be called, got %d calls", calls)
	}
}

func TestDeleteMyPaperRequestsPopupConfirmationBeforeDeleting(t *testing.T) {
	clearPendingPaperDeleteConfirmationsForTest(t)
	var calls int
	withDeletePaperStubs(t, map[string]*model.Paper{
		"p1": {
			ID:       "p1",
			OwnerID:  "stu-1",
			Title:    "BERT: Pre-training of Deep Bidirectional Transformers",
			FileName: "bert-pretraining.pdf",
			Status:   constant.PaperReady,
		},
	}, func(ctx context.Context, ownerID, paperID string) error {
		calls++
		return nil
	})

	var events []core.StreamEvent
	ctx := core.WithStream(paperToolCtx("stu-1"), func(ev core.StreamEvent) {
		events = append(events, ev)
	})
	out, err := deleteMyPaper(ctx, deletePaperInput{PaperID: "p1"})
	if err != nil {
		t.Fatalf("deleteMyPaper: %v", err)
	}
	if calls != 0 {
		t.Fatalf("delete function should not be called before popup confirmation, got %d", calls)
	}
	if out.Status != "confirmation_required" || out.PaperID != "p1" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if len(events) != 1 || events[0].Kind != constant.StreamEventConfirmDeletePaper {
		t.Fatalf("expected confirm_delete_paper event, got %+v", events)
	}
	payload, ok := events[0].Payload.(map[string]string)
	if !ok {
		t.Fatalf("unexpected payload type: %T", events[0].Payload)
	}
	if payload["paper_id"] != "p1" || payload["confirmation_token"] == "" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestDeleteMyPaperDeletesOwnedPaperWithPopupToken(t *testing.T) {
	clearPendingPaperDeleteConfirmationsForTest(t)
	var gotOwner, gotPaper string
	withDeletePaperStubs(t, map[string]*model.Paper{
		"p1": {
			ID:       "p1",
			OwnerID:  "stu-1",
			Title:    "BERT: Pre-training of Deep Bidirectional Transformers",
			FileName: "bert-pretraining.pdf",
			Status:   constant.PaperReady,
		},
	}, func(ctx context.Context, ownerID, paperID string) error {
		gotOwner = ownerID
		gotPaper = paperID
		return nil
	})

	token, err := newPaperDeleteConfirmation("stu-1", "p1")
	if err != nil {
		t.Fatalf("newPaperDeleteConfirmation: %v", err)
	}
	ctx := WithPaperDeleteConfirmation(paperToolCtx("stu-1"), token)
	out, err := deleteMyPaper(ctx, deletePaperInput{PaperID: "p1"})
	if err != nil {
		t.Fatalf("deleteMyPaper: %v", err)
	}
	if gotOwner != "stu-1" || gotPaper != "p1" {
		t.Fatalf("delete function called with owner=%q paper=%q", gotOwner, gotPaper)
	}
	if out.PaperID != "p1" || out.Status != "deleted" || out.Title == "" {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestConfirmPaperDeleteDeletesOwnedPaperWithPopupToken(t *testing.T) {
	clearPendingPaperDeleteConfirmationsForTest(t)
	var gotOwner, gotPaper string
	withDeletePaperStubs(t, map[string]*model.Paper{
		"p1": {
			ID:       "p1",
			OwnerID:  "stu-1",
			Title:    "BERT: Pre-training of Deep Bidirectional Transformers",
			FileName: "bert-pretraining.pdf",
			Status:   constant.PaperReady,
		},
	}, func(ctx context.Context, ownerID, paperID string) error {
		gotOwner = ownerID
		gotPaper = paperID
		return nil
	})

	token, err := newPaperDeleteConfirmation("stu-1", "p1")
	if err != nil {
		t.Fatalf("newPaperDeleteConfirmation: %v", err)
	}
	ctx := WithPaperDeleteConfirmation(paperToolCtx("stu-1"), token)
	title, message, err := ConfirmPaperDelete(ctx, "p1")
	if err != nil {
		t.Fatalf("ConfirmPaperDelete: %v", err)
	}
	if gotOwner != "stu-1" || gotPaper != "p1" {
		t.Fatalf("delete function called with owner=%q paper=%q", gotOwner, gotPaper)
	}
	if title == "" || message == "" {
		t.Fatalf("expected title and message, got title=%q message=%q", title, message)
	}
}

func TestConfirmPaperDeleteRejectsMissingPopupToken(t *testing.T) {
	clearPendingPaperDeleteConfirmationsForTest(t)
	var calls int
	withDeletePaperStubs(t, map[string]*model.Paper{
		"p1": {ID: "p1", OwnerID: "stu-1", FileName: "my-paper.pdf"},
	}, func(context.Context, string, string) error {
		calls++
		return nil
	})

	if _, _, err := ConfirmPaperDelete(paperToolCtx("stu-1"), "p1"); err == nil {
		t.Fatal("missing token should be rejected")
	}
	if calls != 0 {
		t.Fatalf("delete function should not be called, got %d", calls)
	}
}

func TestDeleteMyPaperConsumesPopupTokenOnce(t *testing.T) {
	clearPendingPaperDeleteConfirmationsForTest(t)
	var calls int
	withDeletePaperStubs(t, map[string]*model.Paper{
		"p1": {ID: "p1", OwnerID: "stu-1", FileName: "my-paper.pdf"},
	}, func(context.Context, string, string) error {
		calls++
		return nil
	})

	token, err := newPaperDeleteConfirmation("stu-1", "p1")
	if err != nil {
		t.Fatalf("newPaperDeleteConfirmation: %v", err)
	}
	ctx := WithPaperDeleteConfirmation(paperToolCtx("stu-1"), token)
	if _, err := deleteMyPaper(ctx, deletePaperInput{PaperID: "p1"}); err != nil {
		t.Fatalf("first delete should use token: %v", err)
	}
	if _, err := deleteMyPaper(ctx, deletePaperInput{PaperID: "p1"}); err != nil {
		t.Fatalf("expired token should request a new confirmation, not fail: %v", err)
	}
	if calls != 1 {
		t.Fatalf("delete function should be called once, got %d", calls)
	}
}
