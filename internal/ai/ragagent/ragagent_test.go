package ragagent

import "testing"

// TestEvictUser_OnlyTargetUser 验证复合键(userID|maxIter)下 EvictUser 只清目标用户的全部预算条目,
// 不误伤同前缀的其他用户(如 "u1" 与 "u10")。
func TestEvictUser_OnlyTargetUser(t *testing.T) {
	runners.Store("u1|4", &runnerEntry{})
	runners.Store("u1|6", &runnerEntry{})
	runners.Store("u10|4", &runnerEntry{}) // 前缀相近的另一个用户,不应被清
	t.Cleanup(func() {
		runners.Delete("u1|4")
		runners.Delete("u1|6")
		runners.Delete("u10|4")
	})

	EvictUser("u1")

	if _, ok := runners.Load("u1|4"); ok {
		t.Error("u1|4 应被清除")
	}
	if _, ok := runners.Load("u1|6"); ok {
		t.Error("u1|6 应被清除")
	}
	if _, ok := runners.Load("u10|4"); !ok {
		t.Error("u10|4 是另一个用户,不应被清除")
	}
}
