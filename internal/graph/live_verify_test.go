package graph

import (
	"context"
	"os"
	"testing"

	"GopherPaper/internal/config"
)

// TestLiveGraph 跑真实 Neo4j 端到端验证 Cypher,默认跳过,设 GRAPH_LIVE_URI 才执行。
func TestLiveGraph(t *testing.T) {
	uri := os.Getenv("GRAPH_LIVE_URI")
	if uri == "" {
		t.Skip("未设 GRAPH_LIVE_URI,跳过 live 验证")
	}
	if err := Init(config.Neo4jConfig{URI: uri, Username: "neo4j", Password: "gopherpaper"}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer Close()
	ctx := context.Background()
	const owner = "u1"

	// 清场,保证可重复跑。
	_, _ = exec(ctx, `MATCH (n {owner:$o}) DETACH DELETE n`, map[string]any{"o": owner})

	// 两篇论文:p2 引用 p1 的标题,且共享作者 Alice 与关键词 vla。
	if err := UpsertPaper(ctx, PaperGraph{Owner: owner, ID: "p1", Title: "Deep Residual Learning", Year: 2016, Venue: "CVPR",
		Authors: []string{"Alice", "Bob"}, Keywords: []string{"vla", "vision"}, Affiliations: []string{"MSRA"}}); err != nil {
		t.Fatalf("UpsertPaper p1: %v", err)
	}
	if err := UpsertPaper(ctx, PaperGraph{Owner: owner, ID: "p2", Title: "SafeVLA", Year: 2023, Venue: "NeurIPS",
		Authors: []string{"Alice", "Carol"}, Keywords: []string{"vla", "safety"}, Affiliations: []string{"PKU"}}); err != nil {
		t.Fatalf("UpsertPaper p2: %v", err)
	}
	// p2 的参考文献含 p1 标题,应建出 p2-CITES->p1 直接边 + Reference 桩。
	if err := UpsertCitations(ctx, owner, "p2", []string{
		"He et al. Deep Residual Learning for Image Recognition. CVPR 2016.",
		"Vaswani et al. Attention Is All You Need. NeurIPS 2017.",
	}); err != nil {
		t.Fatalf("UpsertCitations p2: %v", err)
	}
	// 幂等:重复 upsert 不应翻倍边。
	if err := UpsertPaper(ctx, PaperGraph{Owner: owner, ID: "p1", Title: "Deep Residual Learning", Year: 2016, Venue: "CVPR",
		Authors: []string{"Alice", "Bob"}, Keywords: []string{"vla", "vision"}, Affiliations: []string{"MSRA"}}); err != nil {
		t.Fatalf("re-UpsertPaper p1: %v", err)
	}

	stats, err := Overview(ctx, owner)
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if stats.Papers != 2 {
		t.Errorf("Overview.Papers = %d, want 2", stats.Papers)
	}
	if stats.MinYear != 2016 || stats.MaxYear != 2023 {
		t.Errorf("Overview 年份跨度 = %d-%d, want 2016-2023", stats.MinYear, stats.MaxYear)
	}
	if stats.Authors != 3 { // Alice Bob Carol
		t.Errorf("Overview.Authors = %d, want 3", stats.Authors)
	}

	years, err := TrendByYear(ctx, owner)
	if err != nil {
		t.Fatalf("TrendByYear: %v", err)
	}
	if len(years) != 2 {
		t.Errorf("TrendByYear 桶数 = %d, want 2", len(years))
	}

	kt, err := KeywordTrend(ctx, owner, 10)
	if err != nil {
		t.Fatalf("KeywordTrend: %v", err)
	}
	if len(kt) == 0 {
		t.Errorf("KeywordTrend 不应为空")
	}

	tk, err := TopKeywords(ctx, owner, 10)
	if err != nil {
		t.Fatalf("TopKeywords: %v", err)
	}
	if len(tk) == 0 || tk[0].Name != "vla" {
		t.Errorf("TopKeywords[0] 应为 vla,得 %+v", tk)
	}

	rel, err := RelatedPapers(ctx, owner, "p2", 10)
	if err != nil {
		t.Fatalf("RelatedPapers: %v", err)
	}
	if len(rel) != 1 || rel[0].ID != "p1" {
		t.Fatalf("RelatedPapers(p2) 应含 p1,得 %+v", rel)
	}
	// p2 与 p1 共享 author(Alice)+keyword(vla)+直接引用,vias 至少含这三类。
	if len(rel[0].Vias) < 2 {
		t.Errorf("相关关系 vias 偏少: %+v", rel[0].Vias)
	}
	t.Logf("RelatedPapers(p2) = %+v", rel)

	// owner 隔离:换 owner 查应为空。
	other, err := Overview(ctx, "u2")
	if err != nil {
		t.Fatalf("Overview u2: %v", err)
	}
	if other.Papers != 0 {
		t.Errorf("owner 隔离失败,u2 看到 %d 篇", other.Papers)
	}

	// 删除联动:删 p1 后图中应只剩 p2。
	if err := DeletePaper(ctx, owner, "p1"); err != nil {
		t.Fatalf("DeletePaper: %v", err)
	}
	stats2, _ := Overview(ctx, owner)
	if stats2.Papers != 1 {
		t.Errorf("删除后 Papers = %d, want 1", stats2.Papers)
	}
}
