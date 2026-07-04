package core

import (
	"context"
	"testing"
)

func TestComparePaperScopeIsCopiedAndRestricted(t *testing.T) {
	input := []ComparePaperScope{
		{ID: "paper-a", Title: "Paper A"},
		{ID: "paper-b", Title: "Paper B"},
	}
	ctx := WithComparePapers(context.Background(), input)
	input[0].Title = "mutated"

	paper, ok := ComparePaperFrom(ctx, "paper-a")
	if !ok {
		t.Fatal("ComparePaperFrom() did not find allowed paper")
	}
	if paper.Title != "Paper A" {
		t.Fatalf("paper.Title = %q, want copied value", paper.Title)
	}
	if _, ok := ComparePaperFrom(ctx, "paper-c"); ok {
		t.Fatal("ComparePaperFrom() allowed paper outside comparison scope")
	}

	returned := ComparePapersFrom(ctx)
	returned[0].Title = "changed"
	again, _ := ComparePaperFrom(ctx, "paper-a")
	if again.Title != "Paper A" {
		t.Fatal("ComparePapersFrom() exposed mutable context state")
	}
}
