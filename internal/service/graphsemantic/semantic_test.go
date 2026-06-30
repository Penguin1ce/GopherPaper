package graphsemantic

import (
	"strings"
	"testing"

	graphstore "GopherPaper/internal/graph"
)

func TestHeuristicMatchRequiresConcreteFieldEvidence(t *testing.T) {
	safeVLA := graphstore.PaperGraph{
		ID:                "safe-vla",
		Title:             "SafeVLA: Towards Safety Alignment of Vision-Language-Action Model via Constrained Learning",
		Keywords:          []string{"Vision-Language-Action Models", "Safety Alignment", "Constrained Learning"},
		ResearchQuestions: []string{"How to align VLA models for safe robotic action under constraints?"},
		Methods:           "Constrained reinforcement learning with safety costs for VLA policies.",
		Experiments:       "Robot manipulation and safety benchmark tasks.",
	}
	vidFreeze := graphstore.PaperGraph{
		ID:                "vid-freeze",
		Title:             "VID-FREEZE: Protecting Images from Malicious Image-to-Video Generation via Temporal Freezing",
		Keywords:          []string{"Image-to-Video Generation", "Temporal Freezing", "Adversarial Immunization"},
		ResearchQuestions: []string{"How can images be protected from malicious I2V generation?"},
		Methods:           "Adversarial image immunization to suppress temporal motion in diffusion video generation.",
		Experiments:       "CogVideoX image-to-video generation metrics.",
	}

	if match, ok := heuristicMatch(vidFreeze.ID, 0.734, safeVLA, vidFreeze); ok {
		t.Fatalf("unrelated papers should not match, got %+v", match)
	}
}

func TestHeuristicMatchReturnsConcreteFieldEvidence(t *testing.T) {
	a := graphstore.PaperGraph{
		ID:          "a",
		Keywords:    []string{"image-to-video generation", "temporal freezing"},
		Methods:     "Adversarial image immunization suppresses temporal motion.",
		Experiments: "CogVideoX benchmark.",
	}
	b := graphstore.PaperGraph{
		ID:          "b",
		Keywords:    []string{"I2V generation", "temporal freezing"},
		Methods:     "The method uses adversarial image immunization to reduce motion.",
		Experiments: "CogVideoX benchmark.",
	}

	match, ok := heuristicMatch(b.ID, 0.62, a, b)
	if !ok {
		t.Fatal("expected concrete field evidence match")
	}
	if len(match.MatchedFields) == 0 || match.KeywordSimilarity == "" || match.MethodSimilarity == "" {
		t.Fatalf("missing concrete evidence: %+v", match)
	}
	if !strings.Contains(match.MethodSimilarity, "来源：方法") ||
		!strings.Contains(match.MethodSimilarity, "论文A方法片段：") ||
		!strings.Contains(match.MethodSimilarity, "论文B方法片段：") {
		t.Fatalf("method evidence should include its source field: %s", match.MethodSimilarity)
	}
	if !strings.Contains(match.ExperimentSimilarity, "论文A数据集：CogVideoX") ||
		!strings.Contains(match.ExperimentSimilarity, "论文B数据集：CogVideoX") {
		t.Fatalf("experiment evidence should prefer dataset names: %s", match.ExperimentSimilarity)
	}
}

func TestKeywordEvidenceDeduplicatesSharedTerms(t *testing.T) {
	a := graphstore.PaperGraph{
		ID:       "a",
		Keywords: []string{"Transformer"},
	}
	b := graphstore.PaperGraph{
		ID:       "b",
		Keywords: []string{"Transformer", "Pre-LN Transformer", "Post-LN Transformer"},
	}

	match, ok := heuristicMatch(b.ID, 0.75, a, b)
	if !ok {
		t.Fatal("expected keyword evidence match")
	}
	if strings.Contains(match.KeywordSimilarity, "论文A关键词") || strings.Contains(match.KeywordSimilarity, "论文B关键词") {
		t.Fatalf("keyword evidence should be unified, got %q", match.KeywordSimilarity)
	}
	if got := strings.Count(strings.ToLower(match.KeywordSimilarity), "相似关键词：transformer"); got != 1 {
		t.Fatalf("shared transformer evidence should be deduplicated, got %d in %q", got, match.KeywordSimilarity)
	}
}

func TestExperimentEvidenceIgnoresHardwareTerms(t *testing.T) {
	a := graphstore.PaperGraph{
		ID:          "a",
		Experiments: "All experiments run on NVIDIA GPU servers with CUDA acceleration.",
	}
	b := graphstore.PaperGraph{
		ID:          "b",
		Experiments: "Training uses NVIDIA GPUs for efficient Transformer experiments.",
	}

	if got := semanticEvidence(a, b).experiment; got != "" {
		t.Fatalf("hardware terms should not be treated as datasets, got %q", got)
	}
}

func TestMergeJudgeMatchWithEvidenceFillsMissingMethod(t *testing.T) {
	judge := graphstore.SemanticMatch{
		TargetID:          "b",
		Score:             0.39,
		MatchedFields:     []string{"关键词"},
		KeywordSimilarity: "相似关键词：transformer",
		Model:             semanticJudgeModel,
	}
	evidence := graphstore.SemanticMatch{
		KeywordSimilarity: "相似关键词：transformer",
		MethodSimilarity:  "来源：方法\n相似点：位置-wise前馈网络\n论文A方法片段：...\n论文B方法片段：...",
	}

	merged := mergeJudgeMatchWithEvidence(judge, evidence)
	if merged.MethodSimilarity == "" {
		t.Fatalf("expected local method evidence to fill AI-approved relation: %+v", merged)
	}
	if !containsString(merged.MatchedFields, "方法") {
		t.Fatalf("expected matched fields to include method: %+v", merged.MatchedFields)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
