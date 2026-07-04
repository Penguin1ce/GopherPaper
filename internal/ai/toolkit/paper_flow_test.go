package toolkit

import "testing"

func TestNormalizePaperFlowDropsInvalidNodesAndEdges(t *testing.T) {
	flow, err := normalizePaperFlow(paperFlow{
		Title: " test ",
		Nodes: []flowNode{
			{ID: "n1", Type: "problem", Label: "研究问题"},
			{ID: "n2", Type: "gap", Label: "现有不足"},
			{ID: "n2", Type: "idea", Label: "重复节点"},
			{ID: "n3", Type: "idea", Label: "核心思路"},
			{ID: "n4", Type: "method", Label: "方法设计"},
			{ID: "n5", Type: "experiment", Label: "实验验证"},
			{ID: "n6", Type: "result", Label: "关键结果"},
			{ID: "bad", Type: "other", Label: "非法类型"},
		},
		Edges: []flowEdge{
			{From: "n1", To: "n2", Label: "因此"},
			{From: "n1", To: "n2", Label: "重复"},
			{From: "n2", To: "missing", Label: "非法"},
			{From: "n3", To: "n3", Label: "自环"},
		},
	})
	if err != nil {
		t.Fatalf("normalizePaperFlow: %v", err)
	}
	if flow.Title != "test" {
		t.Fatalf("title not trimmed: %q", flow.Title)
	}
	if len(flow.Nodes) != 6 {
		t.Fatalf("expected 6 valid nodes, got %d", len(flow.Nodes))
	}
	if len(flow.Edges) != 1 || flow.Edges[0].From != "n1" || flow.Edges[0].To != "n2" {
		t.Fatalf("unexpected edges: %+v", flow.Edges)
	}
}

func TestNormalizePaperFlowRequiresMinimumNodes(t *testing.T) {
	_, err := normalizePaperFlow(paperFlow{
		Nodes: []flowNode{
			{ID: "n1", Type: "problem", Label: "研究问题"},
			{ID: "n2", Type: "gap", Label: "现有不足"},
		},
	})
	if err == nil {
		t.Fatal("too few nodes should be rejected")
	}
}
