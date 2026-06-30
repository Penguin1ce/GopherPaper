package graph

import (
	"testing"
	"time"

	paperdao "GopherPaper/internal/dao/paper"
)

func TestPapersNeedingSync(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	graphState := map[string]int64{
		"fresh": base.UnixMilli(), // 图谱与 meta 同时刻 → 不需同步
		"stale": base.UnixMilli(), // meta 更晚 → 需同步
	}
	stamps := []paperdao.MetaStamp{
		{PaperID: "fresh", UpdatedAt: base},
		{PaperID: "stale", UpdatedAt: base.Add(time.Minute)},
		{PaperID: "missing", UpdatedAt: base}, // 图谱无此节点 → 需同步
	}
	got := papersNeedingSync(graphState, stamps)
	want := map[string]bool{"stale": true, "missing": true}
	if len(got) != len(want) {
		t.Fatalf("papersNeedingSync = %v, want 键 %v", got, want)
	}
	for _, id := range got {
		if !want[id] {
			t.Fatalf("papersNeedingSync 多返回了 %q, got %v", id, got)
		}
	}
}

func TestPapersNeedingSyncAllFresh(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	graphState := map[string]int64{"a": base.UnixMilli()}
	stamps := []paperdao.MetaStamp{{PaperID: "a", UpdatedAt: base.Add(-time.Second)}}
	if got := papersNeedingSync(graphState, stamps); len(got) != 0 {
		t.Fatalf("papersNeedingSync = %v, want 空", got)
	}
}
