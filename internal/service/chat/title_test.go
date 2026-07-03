package chat

import (
	"testing"

	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
)

func TestShouldRewriteSessionTitleOnlyFirstBoundDefaultSession(t *testing.T) {
	sess := &model.Session{PaperID: "paper-1"}
	if !shouldRewriteSessionTitle(sess, nil) {
		t.Fatal("绑定论文的默认会话首轮应改写标题")
	}

	withUserHistory := []model.Message{{Role: model.RoleUser, Content: "第一问"}}
	if shouldRewriteSessionTitle(sess, withUserHistory) {
		t.Fatal("已有用户消息的会话不应再次改写标题")
	}

	if shouldRewriteSessionTitle(&model.Session{}, nil) {
		t.Fatal("未绑定论文的会话不应按论文会话标题规则改写")
	}

	if shouldRewriteSessionTitle(&model.Session{PaperID: "paper-1", AgentType: constant.AgentPioneer}, nil) {
		t.Fatal("非小文鸮默认会话不应改写标题")
	}
}
