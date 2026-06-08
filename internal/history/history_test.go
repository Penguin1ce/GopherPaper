package history

import (
	"context"
	"testing"

	mysqlsession "trpc.group/trpc-go/trpc-agent-go/session/mysql"

	"GopherPaper/internal/config"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
)

// initMySQL 用真实 MySQL（读项目 config）起一套 Session 服务，连不上则 skip。
// 用独立表前缀 test_history_ 与生产 session_* 表隔离，不污染业务数据。
func initMySQL(t *testing.T) {
	t.Helper()
	cfg, err := config.Load("../../config/config.toml")
	if err != nil {
		t.Skipf("跳过：未找到可用 config.toml: %v", err)
	}
	s, err := mysqlsession.NewService(
		mysqlsession.WithMySQLClientDSN(cfg.MySQL.DSN),
		mysqlsession.WithTablePrefix("test_history_"),
	)
	if err != nil {
		t.Skipf("跳过：MySQL 不可用: %v", err)
	}
	svc = s
}

// TestAppendListRoundtrip 验证一轮对话落 Session 后能按序还原，意图随助教消息透传。
func TestAppendListRoundtrip(t *testing.T) {
	initMySQL(t)
	ctx := context.Background()
	const user, sess = "stu-1", "hist-test-1"
	t.Cleanup(func() { _ = Delete(ctx, user, sess) })

	user1 := &model.Message{SessionID: sess, Role: model.RoleUser, Content: "这篇论文的方法是什么"}
	ai1 := &model.Message{
		SessionID: sess, Role: model.RoleAssistant, Content: "采用了对比学习", Intent: constant.IntentMethod,
		Meta: map[string]any{"sources": []any{map[string]any{"doc_id": "p1", "block_type": "image", "img_name": "fig1.jpg"}}},
	}
	if err := Append(ctx, user, sess, user1, ai1); err != nil {
		t.Fatalf("Append 失败: %v", err)
	}

	msgs, err := List(ctx, user, sess)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("期望 2 条，得到 %d 条", len(msgs))
	}
	if msgs[0].Role != model.RoleUser || msgs[0].Content != "这篇论文的方法是什么" {
		t.Fatalf("第 1 条还原错误: %+v", msgs[0])
	}
	if msgs[1].Role != model.RoleAssistant || msgs[1].Intent != constant.IntentMethod {
		t.Fatalf("助教消息或意图还原错误: %+v", msgs[1])
	}
	if msgs[1].ID == "" {
		t.Fatal("还原的消息应带 Session 事件 ID")
	}
	// 出处 meta 须经 Session 事件 Extensions 持久化并还原,退出重进后引用与内联图才不丢。
	srcs, ok := msgs[1].Meta["sources"].([]any)
	if !ok || len(srcs) != 1 {
		t.Fatalf("出处 meta 未随历史还原: %+v", msgs[1].Meta)
	}
	if got := srcs[0].(map[string]any)["img_name"]; got != "fig1.jpg" {
		t.Fatalf("出处图片名还原错误: %v", got)
	}
}

// TestLoadWindow 验证 Load 只取最近 limit 条作上下文窗口。
func TestLoadWindow(t *testing.T) {
	initMySQL(t)
	ctx := context.Background()
	const user, sess = "stu-2", "hist-test-2"
	t.Cleanup(func() { _ = Delete(ctx, user, sess) })

	for i := 0; i < 3; i++ {
		u := &model.Message{SessionID: sess, Role: model.RoleUser, Content: "q"}
		a := &model.Message{SessionID: sess, Role: model.RoleAssistant, Content: "a"}
		if err := Append(ctx, user, sess, u, a); err != nil {
			t.Fatalf("Append 失败: %v", err)
		}
	}
	last2, err := Load(ctx, user, sess, 2)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if len(last2) != 2 {
		t.Fatalf("期望窗口 2 条，得到 %d 条", len(last2))
	}
}

// TestIsolationAndDelete 验证会话间隔离与删除清空历史。
func TestIsolationAndDelete(t *testing.T) {
	initMySQL(t)
	ctx := context.Background()
	const ua, sa = "stu-a", "hist-test-a"
	const ub, sb = "stu-b", "hist-test-b"
	t.Cleanup(func() { _ = Delete(ctx, ua, sa); _ = Delete(ctx, ub, sb) })

	if err := Append(ctx, ua, sa, &model.Message{Role: model.RoleUser, Content: "a"}); err != nil {
		t.Fatalf("Append a 失败: %v", err)
	}
	if err := Append(ctx, ub, sb, &model.Message{Role: model.RoleUser, Content: "b"}); err != nil {
		t.Fatalf("Append b 失败: %v", err)
	}

	// 另一会话读不到本会话历史。
	other, _ := List(ctx, ua, sb)
	if len(other) != 0 {
		t.Fatalf("会话隔离失败，读到 %d 条", len(other))
	}

	if err := Delete(ctx, ua, sa); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	gone, _ := List(ctx, ua, sa)
	if len(gone) != 0 {
		t.Fatalf("删除后仍有 %d 条历史", len(gone))
	}
}

// TestLoadMissingSession 验证未创建的会话读历史返回空而非报错。
func TestLoadMissingSession(t *testing.T) {
	initMySQL(t)
	msgs, err := Load(context.Background(), "nobody", "hist-test-no-such", 10)
	if err != nil {
		t.Fatalf("读不存在会话应无错: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("期望空，得到 %d 条", len(msgs))
	}
}
