package chat

import (
	"context"
	"errors"
	"testing"

	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

func TestCreateSessionRequiresPaperForMaodie(t *testing.T) {
	_, err := CreateSession(context.Background(), "stu-1", "  ", "小耄耋", constant.AgentMaodie)
	if !errors.Is(err, errs.ErrPaperRequired) {
		t.Fatalf("err=%v, want ErrPaperRequired", err)
	}
}
