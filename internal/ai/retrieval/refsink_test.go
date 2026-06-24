package retrieval

import (
	"context"
	"testing"
)

func TestRefSink_AccumulateDedupDrain(t *testing.T) {
	ctx := WithRefSink(context.Background())
	AddRefs(ctx, []Reference{{ID: "a"}, {ID: "b"}})
	AddRefs(ctx, []Reference{{ID: "b"}, {ID: "c"}}) // b 重复应去掉
	got := DrainRefs(ctx)
	if len(got) != 3 {
		t.Fatalf("应去重后剩 3 条, got %d: %+v", len(got), got)
	}
	want := []string{"a", "b", "c"} // 保留首次出现顺序
	for i, r := range got {
		if r.ID != want[i] {
			t.Errorf("第 %d 条 = %q, want %q", i, r.ID, want[i])
		}
	}
	// drain 后应清空,再取为空。
	if again := DrainRefs(ctx); len(again) != 0 {
		t.Errorf("drain 后应清空, got %+v", again)
	}
}

func TestRefSink_NoSinkIsNoop(t *testing.T) {
	// 固定流 ctx 不挂收集器:AddRefs 静默忽略,DrainRefs 返回 nil,均不 panic。
	ctx := context.Background()
	AddRefs(ctx, []Reference{{ID: "x"}})
	if got := DrainRefs(ctx); got != nil {
		t.Errorf("无收集器应返回 nil, got %+v", got)
	}
}
