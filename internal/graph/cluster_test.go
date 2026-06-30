package graph

import (
	"reflect"
	"testing"
)

func TestDetectCommunitiesTwoClusters(t *testing.T) {
	// {a,b} 与 {c,d} 各自抱团,团间无边;e 孤立。
	nodes := []string{"a", "b", "c", "d", "e"}
	edges := []weightedEdge{{A: "a", B: "b", W: 5}, {A: "c", B: "d", W: 5}}
	got := detectCommunities(nodes, edges)
	if got["a"] != got["b"] {
		t.Errorf("a,b 应同社区, got a=%d b=%d", got["a"], got["b"])
	}
	if got["c"] != got["d"] {
		t.Errorf("c,d 应同社区, got c=%d d=%d", got["c"], got["d"])
	}
	if got["a"] == got["c"] {
		t.Errorf("{a,b} 与 {c,d} 应不同社区")
	}
	if got["e"] == got["a"] || got["e"] == got["c"] {
		t.Errorf("孤立点 e 应自成一团, got e=%d", got["e"])
	}
}

func TestDetectCommunitiesTriangleOneCluster(t *testing.T) {
	nodes := []string{"a", "b", "c"}
	edges := []weightedEdge{{A: "a", B: "b", W: 1}, {A: "b", B: "c", W: 1}, {A: "a", B: "c", W: 1}}
	got := detectCommunities(nodes, edges)
	if got["a"] != got["b"] || got["b"] != got["c"] {
		t.Errorf("三角全连通应同社区, got %v", got)
	}
}

func TestDetectCommunitiesDeterministic(t *testing.T) {
	nodes := []string{"a", "b", "c", "d"}
	edges := []weightedEdge{{A: "a", B: "b", W: 2}, {A: "c", B: "d", W: 2}, {A: "b", B: "c", W: 1}}
	first := detectCommunities(nodes, edges)
	for i := 0; i < 5; i++ {
		if got := detectCommunities(nodes, edges); !reflect.DeepEqual(got, first) {
			t.Fatalf("结果不确定: %v != %v", got, first)
		}
	}
}
