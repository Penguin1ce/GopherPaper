package graph

import (
	"encoding/json"
	"testing"
)

func TestEmptyEntityGraphJSONUsesArrays(t *testing.T) {
	b, err := json.Marshal(emptyEntityGraph())
	if err != nil {
		t.Fatalf("marshal empty graph: %v", err)
	}
	got := string(b)
	want := `{"nodes":[],"edges":[]}`
	if got != want {
		t.Fatalf("empty graph json = %s, want %s", got, want)
	}
}
