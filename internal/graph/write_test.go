package graph

import (
	"strings"
	"testing"
)

func TestNormalizeTitle(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Attention Is All You Need", "attention is all you need"},
		{"  Deep   Residual\tLearning ", "deep residual learning"},
		{"BERT: Pre-training of Deep Bidirectional...", "bert pre training of deep bidirectional"},
		{"[12] Smith et al., 2021.", "12 smith et al 2021"},
		{"", ""},
		{"---", ""},
	}
	for _, c := range cases {
		if got := normalizeTitle(c.in); got != c.want {
			t.Errorf("normalizeTitle(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeTitleStableForCoCitation(t *testing.T) {
	// 同一条参考文献的不同排版应归一化到同一键,保证共被引能连上。
	a := normalizeTitle("He, K. et al. Deep Residual Learning for Image Recognition. CVPR 2016.")
	b := normalizeTitle("He K, et al.  deep residual learning for image recognition,  CVPR, 2016")
	if a != b {
		t.Errorf("同义参考文献归一化不一致:\n a=%q\n b=%q", a, b)
	}
}

func TestCleanTerms(t *testing.T) {
	// 大小写/空格差异按 norm 合并成同一项,展示名取首次出现的原文,空项丢弃。
	got := cleanTerms([]string{"Transformer", "transformer", " Self-Attention ", "", "  ", "self attention"})
	want := []struct{ name, norm string }{
		{"Transformer", "transformer"},
		{"Self-Attention", "self attention"},
	}
	if len(got) != len(want) {
		t.Fatalf("cleanTerms 长度 = %d, want %d (%v)", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i]["name"] != w.name || got[i]["norm"] != w.norm {
			t.Errorf("cleanTerms[%d] = %v, want {name:%q norm:%q}", i, got[i], w.name, w.norm)
		}
	}
}

func TestRelinkSimilarDeleteKeepsInboundEdges(t *testing.T) {
	if want := "-[s:SIMILAR_TO]->() DELETE s"; !containsCypher(relinkSimilarDeleteCypher, want) {
		t.Fatalf("相似边重建只应删除当前论文出边, cypher=%q", relinkSimilarDeleteCypher)
	}
	if containsCypher(relinkSimilarDeleteCypher, "-[s:SIMILAR_TO]-()") {
		t.Fatalf("相似边重建不应删除双向边, cypher=%q", relinkSimilarDeleteCypher)
	}
}

func containsCypher(s, sub string) bool {
	return strings.Contains(strings.Join(strings.Fields(s), " "), sub)
}
