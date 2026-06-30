package graphsemantic

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"GopherPaper/internal/ai/agentrt"
	paperdao "GopherPaper/internal/dao/paper"
	graphstore "GopherPaper/internal/graph"
	"GopherPaper/internal/knowledge"
	"GopherPaper/internal/model"
	"GopherPaper/internal/zlog"
)

const (
	semanticCandidateTopK        = 16
	semanticAllPairsFallbackTopK = 24
	semanticRecallScore          = 0.06
	semanticAcceptScore          = 0.30
	semanticJudgeModel           = "agentrt-chat"
)

var datasetNamePattern = regexp.MustCompile(`\b[A-Z][A-Za-z0-9]*(?:[-_][A-Za-z0-9]+)*(?:\s+\d{2,4})?\b`)

type profileInput struct {
	Title             string   `json:"title"`
	Keywords          []string `json:"keywords"`
	ResearchQuestions []string `json:"research_questions"`
	Methods           string   `json:"methods"`
	Experiments       string   `json:"experiments"`
}

type judgeInput struct {
	PaperA            profileInput          `json:"paper_a"`
	PaperB            profileInput          `json:"paper_b"`
	CandidateEvidence semanticEvidenceInput `json:"candidate_evidence"`
}

type semanticEvidenceInput struct {
	MatchedFields              []string `json:"matched_fields"`
	KeywordSimilarity          string   `json:"keyword_similarity"`
	ResearchQuestionSimilarity string   `json:"research_question_similarity"`
	MethodSimilarity           string   `json:"method_similarity"`
	ExperimentSimilarity       string   `json:"experiment_similarity"`
}

type judgeOutput struct {
	IsSimilar                  bool     `json:"is_similar"`
	Score                      float64  `json:"score"`
	MatchedFields              []string `json:"matched_fields"`
	Summary                    string   `json:"summary"`
	KeywordSimilarity          string   `json:"keyword_similarity"`
	ResearchQuestionSimilarity string   `json:"research_question_similarity"`
	MethodSimilarity           string   `json:"method_similarity"`
	ExperimentSimilarity       string   `json:"experiment_similarity"`
}

type semanticFieldEvidence struct {
	fields           []string
	keyword          string
	researchQuestion string
	method           string
	experiment       string
}

func PreparePaperGraph(ctx context.Context, pg graphstore.PaperGraph) graphstore.PaperGraph {
	profile := graphstore.BuildPaperSemanticProfile(pg)
	pg.SemanticProfile = profile.Text
	if pg.SemanticProfile == "" {
		return pg
	}
	vec, err := knowledge.Embed(ctx, pg.SemanticProfile)
	if err != nil {
		zlog.Error("paper semantic profile embedding failed, skip semantic recall", "paper_id", pg.ID, "err", err)
		return pg
	}
	pg.SemanticEmbedding = vec
	if len(pg.Embedding) == 0 {
		pg.Embedding = vec
	}
	return pg
}

func RefreshPaper(ctx context.Context, owner, paperID string) error {
	source, err := paperGraphFromMeta(ctx, owner, paperID)
	if err != nil {
		return err
	}
	source = PreparePaperGraph(ctx, source)
	if err := graphstore.UpsertPaperMetadata(ctx, source); err != nil {
		return err
	}
	return RefreshPaperRelations(ctx, owner, paperID)
}

func RefreshPaperRelations(ctx context.Context, owner, paperID string) error {
	return refreshPaperRelations(ctx, owner, paperID, nil)
}

func RefreshPaperRelationsUnique(ctx context.Context, owner, paperID string, seenPairs map[string]bool) error {
	return refreshPaperRelations(ctx, owner, paperID, seenPairs)
}

func refreshPaperRelations(ctx context.Context, owner, paperID string, seenPairs map[string]bool) error {
	source, err := paperGraphFromMeta(ctx, owner, paperID)
	if err != nil {
		return err
	}
	candidates, err := graphstore.SemanticCandidates(ctx, owner, paperID, semanticCandidateTopK, semanticRecallScore)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		candidates, err = allOwnerCandidates(ctx, owner, paperID)
		if err != nil {
			return err
		}
	}

	seenTargets := map[string]bool{}
	matches := make([]graphstore.SemanticMatch, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ID == "" || candidate.ID == paperID || seenTargets[candidate.ID] {
			continue
		}
		seenTargets[candidate.ID] = true
		if seenPairs != nil {
			key := semanticPairKey(paperID, candidate.ID)
			if seenPairs[key] {
				continue
			}
			seenPairs[key] = true
		}
		target, err := paperGraphFromMeta(ctx, owner, candidate.ID)
		if err != nil {
			zlog.Error("load semantic candidate meta failed, skip", "paper_id", paperID, "target_id", candidate.ID, "err", err)
			continue
		}
		match, ok := judgeSemanticSimilarity(ctx, source, target, candidate.Score)
		if !ok {
			continue
		}
		matches = append(matches, match)
	}
	if err := graphstore.UpsertSemanticSimilar(ctx, owner, paperID, matches); err != nil {
		return err
	}
	if err := saveSemanticRelations(ctx, owner, paperID, matches); err != nil {
		return err
	}
	zlog.Info("semantic graph relations refreshed", "paper_id", paperID, "candidates", len(candidates), "matches", len(matches))
	return nil
}

func semanticPairKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "\x00" + b
}

func allOwnerCandidates(ctx context.Context, owner, paperID string) ([]graphstore.SemanticCandidate, error) {
	papers, err := paperdao.List(ctx, owner)
	if err != nil {
		return nil, err
	}
	out := make([]graphstore.SemanticCandidate, 0, len(papers))
	for _, p := range papers {
		if p.ID == paperID {
			continue
		}
		out = append(out, graphstore.SemanticCandidate{ID: p.ID, Title: firstNonEmpty(p.Title, p.FileName, p.ID)})
		if len(out) >= semanticAllPairsFallbackTopK {
			break
		}
	}
	return out, nil
}

func paperGraphFromMeta(ctx context.Context, owner, paperID string) (graphstore.PaperGraph, error) {
	p, err := paperdao.Get(ctx, paperID)
	if err != nil {
		return graphstore.PaperGraph{}, err
	}
	if p.OwnerID != owner {
		return graphstore.PaperGraph{}, fmt.Errorf("graphsemantic: paper owner mismatch")
	}
	meta, err := paperdao.GetMeta(ctx, paperID)
	if err != nil {
		return graphstore.PaperGraph{}, err
	}
	return graphstore.PaperGraph{
		Owner:             owner,
		ID:                paperID,
		Title:             firstNonEmpty(p.Title, p.FileName, paperID),
		Year:              meta.PublishYear,
		Venue:             meta.Venue,
		Authors:           []string(meta.Authors),
		Keywords:          []string(meta.Keywords),
		Affiliations:      []string(meta.Affiliations),
		ResearchQuestions: []string(meta.ResearchQuestions),
		Methods:           meta.Methods,
		Experiments:       meta.Experiments,
		Results:           meta.Results,
		Innovations:       []string(meta.Innovations),
		Limitations:       []string(meta.Limitations),
		FutureWork:        []string(meta.FutureWork),
	}, nil
}

func judgeSemanticSimilarity(ctx context.Context, source, target graphstore.PaperGraph, recallScore float64) (graphstore.SemanticMatch, bool) {
	fallback, fallbackOK := heuristicMatch(target.ID, recallScore, source, target)
	in := judgeInput{
		PaperA: profileOf(source),
		PaperB: profileOf(target),
	}
	if fallbackOK {
		in.CandidateEvidence = semanticEvidenceForJudge(fallback)
	}
	payload, _ := json.MarshalIndent(in, "", "  ")
	content, err := agentrt.Generate(ctx, semanticJudgePrompt, nil, string(payload))
	if err != nil {
		zlog.Error("semantic relation LLM judge failed, use field evidence fallback", "paper_id", source.ID, "target_id", target.ID, "err", err)
		return fallback, fallbackOK
	}
	var out judgeOutput
	if err := json.Unmarshal([]byte(extractJSON(content)), &out); err != nil {
		zlog.Error("semantic relation LLM JSON parse failed, use field evidence fallback", "paper_id", source.ID, "target_id", target.ID, "err", err)
		return fallback, fallbackOK
	}
	score := out.Score
	if score <= 0 {
		score = recallScore
	}
	matchedFields := acceptedMatchedFields(out)
	if !out.IsSimilar || score < semanticAcceptScore || len(matchedFields) == 0 {
		return graphstore.SemanticMatch{}, false
	}
	match := graphstore.SemanticMatch{
		TargetID:                   target.ID,
		Score:                      score,
		MatchedFields:              matchedFields,
		Summary:                    strings.TrimSpace(out.Summary),
		KeywordSimilarity:          strings.TrimSpace(out.KeywordSimilarity),
		ResearchQuestionSimilarity: strings.TrimSpace(out.ResearchQuestionSimilarity),
		MethodSimilarity:           strings.TrimSpace(out.MethodSimilarity),
		ExperimentSimilarity:       strings.TrimSpace(out.ExperimentSimilarity),
		Model:                      semanticJudgeModel,
	}
	if fallbackOK {
		match = mergeJudgeMatchWithEvidence(match, fallback)
	}
	return match, true
}

func semanticEvidenceForJudge(match graphstore.SemanticMatch) semanticEvidenceInput {
	return semanticEvidenceInput{
		MatchedFields:              nonEmpty(match.MatchedFields),
		KeywordSimilarity:          strings.TrimSpace(match.KeywordSimilarity),
		ResearchQuestionSimilarity: strings.TrimSpace(match.ResearchQuestionSimilarity),
		MethodSimilarity:           strings.TrimSpace(match.MethodSimilarity),
		ExperimentSimilarity:       strings.TrimSpace(match.ExperimentSimilarity),
	}
}

func mergeJudgeMatchWithEvidence(match, evidence graphstore.SemanticMatch) graphstore.SemanticMatch {
	if match.KeywordSimilarity == "" && evidence.KeywordSimilarity != "" {
		match.KeywordSimilarity = evidence.KeywordSimilarity
	}
	if match.ResearchQuestionSimilarity == "" && evidence.ResearchQuestionSimilarity != "" {
		match.ResearchQuestionSimilarity = evidence.ResearchQuestionSimilarity
	}
	if match.MethodSimilarity == "" && evidence.MethodSimilarity != "" {
		match.MethodSimilarity = evidence.MethodSimilarity
	}
	if match.ExperimentSimilarity == "" && evidence.ExperimentSimilarity != "" {
		match.ExperimentSimilarity = evidence.ExperimentSimilarity
	}
	match.MatchedFields = acceptedMatchedFields(judgeOutput{
		KeywordSimilarity:          match.KeywordSimilarity,
		ResearchQuestionSimilarity: match.ResearchQuestionSimilarity,
		MethodSimilarity:           match.MethodSimilarity,
		ExperimentSimilarity:       match.ExperimentSimilarity,
	})
	return match
}

func acceptedMatchedFields(out judgeOutput) []string {
	var fields []string
	if strings.TrimSpace(out.KeywordSimilarity) != "" {
		fields = append(fields, "关键词")
	}
	if strings.TrimSpace(out.ResearchQuestionSimilarity) != "" {
		fields = append(fields, "研究问题")
	}
	if strings.TrimSpace(out.MethodSimilarity) != "" {
		fields = append(fields, "方法")
	}
	if strings.TrimSpace(out.ExperimentSimilarity) != "" {
		fields = append(fields, "实验/数据集")
	}
	return fields
}

func heuristicMatch(targetID string, recallScore float64, source, target graphstore.PaperGraph) (graphstore.SemanticMatch, bool) {
	evidence := semanticEvidence(source, target)
	if len(evidence.fields) == 0 {
		return graphstore.SemanticMatch{}, false
	}
	summary := "两篇论文在" + strings.Join(evidence.fields, "、") + "上存在可定位的相似片段。"
	if recallScore > 0 {
		summary = fmt.Sprintf("%s 候选召回相似度为 %.3f。", summary, recallScore)
	}
	return graphstore.SemanticMatch{
		TargetID:                   targetID,
		Score:                      recallScore,
		MatchedFields:              nonEmpty(evidence.fields),
		Summary:                    summary,
		KeywordSimilarity:          evidence.keyword,
		ResearchQuestionSimilarity: evidence.researchQuestion,
		MethodSimilarity:           evidence.method,
		ExperimentSimilarity:       evidence.experiment,
		Model:                      "field-evidence-fallback",
	}, true
}

func semanticEvidence(source, target graphstore.PaperGraph) semanticFieldEvidence {
	var e semanticFieldEvidence
	if v := keywordEvidence(source.Keywords, target.Keywords); v != "" {
		e.fields = append(e.fields, "关键词")
		e.keyword = v
	}
	if v := listEvidence("研究问题", "论文A研究问题", "论文B研究问题", source.ResearchQuestions, target.ResearchQuestions, 1); v != "" {
		e.fields = append(e.fields, "研究问题")
		e.researchQuestion = v
	}
	if v := textEvidence("方法", "论文A方法片段", "论文B方法片段", source.Methods, target.Methods, 1); v != "" {
		e.fields = append(e.fields, "方法")
		e.method = v
	}
	if v := experimentEvidence(source.Experiments, target.Experiments); v != "" {
		e.fields = append(e.fields, "实验/数据集")
		e.experiment = v
	}
	return e
}

func keywordEvidence(a, b []string) string {
	pairs := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, left := range nonEmpty(a) {
		for _, right := range nonEmpty(b) {
			shared := evidenceSharedTokens(left, right, 1)
			if len(shared) == 0 {
				continue
			}
			key := strings.Join(shared, "|")
			if seen[key] {
				continue
			}
			seen[key] = true
			pairs = append(pairs, "相似关键词："+strings.Join(shared, "、"))
			if len(pairs) >= 3 {
				return strings.Join(pairs, "\n")
			}
		}
	}
	return strings.Join(pairs, "\n")
}

func listEvidence(field, leftLabel, rightLabel string, a, b []string, minShared int) string {
	pairs := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, left := range nonEmpty(a) {
		for _, right := range nonEmpty(b) {
			if evidence, key := evidencePair(field, leftLabel, rightLabel, left, right, minShared); evidence != "" && !seen[key] {
				seen[key] = true
				pairs = append(pairs, evidence)
				if len(pairs) >= 3 {
					return strings.Join(pairs, "\n")
				}
			}
		}
	}
	return strings.Join(pairs, "\n")
}

func textEvidence(field, leftLabel, rightLabel, a, b string, minShared int) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return ""
	}
	leftParts := evidenceSegments(a)
	rightParts := evidenceSegments(b)
	pairs := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, left := range leftParts {
		for _, right := range rightParts {
			if evidence, key := evidencePair(field, leftLabel, rightLabel, left, right, minShared); evidence != "" && !seen[key] {
				seen[key] = true
				pairs = append(pairs, evidence)
				if len(pairs) >= 3 {
					return strings.Join(pairs, "\n")
				}
			}
		}
	}
	return strings.Join(pairs, "\n")
}

func experimentEvidence(a, b string) string {
	leftDatasets := datasetTerms(a)
	rightDatasets := datasetTerms(b)
	if len(leftDatasets) > 0 && len(rightDatasets) > 0 {
		if evidence := listEvidence("实验/数据集", "论文A数据集", "论文B数据集", leftDatasets, rightDatasets, 1); evidence != "" {
			return evidence
		}
	}
	return ""
}

func evidencePair(field, leftLabel, rightLabel, a, b string, minShared int) (string, string) {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return "", ""
	}
	shared := evidenceSharedTokens(a, b, minShared)
	if len(shared) == 0 {
		return "", ""
	}
	key := field + ":" + strings.Join(shared, "|")
	return fmt.Sprintf("来源：%s\n相似点：%s\n%s：%s\n%s：%s", field, strings.Join(shared, "、"), leftLabel, shortEvidence(a), rightLabel, shortEvidence(b)), key
}

func evidenceSharedTokens(a, b string, minShared int) []string {
	shared := sharedSemanticTokens(a, b)
	if !semanticTextsSimilar(a, b) && len(shared) < minShared {
		return nil
	}
	if len(shared) == 0 {
		shared = []string{shortEvidence(a)}
	}
	shared = meaningfulSharedTokens(shared)
	if len(shared) == 0 {
		return nil
	}
	if len(shared) > 5 {
		shared = shared[:5]
	}
	return shared
}

func semanticTextsSimilar(a, b string) bool {
	na := semanticNorm(a)
	nb := semanticNorm(b)
	if na == "" || nb == "" {
		return false
	}
	if na == nb {
		return true
	}
	shorter, longer := na, nb
	if len([]rune(shorter)) > len([]rune(longer)) {
		shorter, longer = longer, shorter
	}
	if len([]rune(shorter)) >= 8 && strings.Contains(longer, shorter) {
		return true
	}
	return tokenOverlap(na, nb) >= 0.28
}

func tokenOverlap(a, b string) float64 {
	ta := semanticTokens(a)
	tb := semanticTokens(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	seen := map[string]bool{}
	for _, token := range ta {
		seen[token] = true
	}
	shared := 0
	for _, token := range tb {
		if seen[token] {
			shared++
		}
	}
	denom := len(ta)
	if len(tb) < denom {
		denom = len(tb)
	}
	if denom == 0 {
		return 0
	}
	return float64(shared) / float64(denom)
}

func semanticTokens(s string) []string {
	raw := strings.Fields(s)
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, token := range raw {
		if len([]rune(token)) >= 3 && !semanticStopword(token) && !seen[token] {
			seen[token] = true
			out = append(out, token)
		}
	}
	return out
}

func sharedSemanticTokens(a, b string) []string {
	ta := semanticTokens(semanticNorm(a))
	tb := semanticTokens(semanticNorm(b))
	if len(ta) == 0 || len(tb) == 0 {
		return nil
	}
	seen := map[string]bool{}
	for _, token := range ta {
		seen[token] = true
	}
	out := make([]string, 0, len(tb))
	used := map[string]bool{}
	for _, token := range tb {
		if seen[token] && !used[token] {
			used[token] = true
			out = append(out, token)
		}
	}
	return out
}

func meaningfulSharedTokens(tokens []string) []string {
	out := make([]string, 0, len(tokens))
	seen := map[string]bool{}
	for _, token := range tokens {
		token = strings.TrimSpace(strings.ToLower(token))
		if token == "" || semanticStopword(token) || seen[token] {
			continue
		}
		seen[token] = true
		out = append(out, token)
	}
	return out
}

func datasetTerms(s string) []string {
	candidates := datasetNamePattern.FindAllString(s, -1)
	out := make([]string, 0, len(candidates))
	seen := map[string]bool{}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		norm := semanticNorm(candidate)
		if norm == "" || seen[norm] || datasetStopword(norm) {
			continue
		}
		if len([]rune(norm)) < 3 {
			continue
		}
		if !datasetLikeName(candidate) && !hasDatasetContext(s, candidate) {
			continue
		}
		seen[norm] = true
		out = append(out, candidate)
	}
	return out
}

func datasetLikeName(s string) bool {
	norm := semanticNorm(s)
	if norm == "" || datasetStopword(norm) {
		return false
	}
	switch {
	case strings.Contains(norm, "cifar"),
		strings.Contains(norm, "imagenet"),
		strings.Contains(norm, "mnist"),
		strings.Contains(norm, "coco"),
		strings.Contains(norm, "squad"),
		strings.Contains(norm, "glue"),
		strings.Contains(norm, "superglue"),
		strings.Contains(norm, "wmt"),
		strings.Contains(norm, "iwslt"),
		strings.Contains(norm, "conll"),
		strings.Contains(norm, "cityscapes"),
		strings.Contains(norm, "kitti"),
		strings.Contains(norm, "waymo"),
		strings.Contains(norm, "nuscenes"),
		strings.Contains(norm, "kinetics"),
		strings.Contains(norm, "ucf"),
		strings.Contains(norm, "hmdb"):
		return true
	}
	hasLower := false
	hasUpper := false
	hasDigit := false
	for _, r := range s {
		if unicode.IsLower(r) {
			hasLower = true
		}
		if unicode.IsUpper(r) {
			hasUpper = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	return hasLower && hasUpper && hasDigit
}

func hasDatasetContext(text, candidate string) bool {
	idx := strings.Index(text, candidate)
	if idx < 0 {
		return false
	}
	start := idx - 80
	if start < 0 {
		start = 0
	}
	end := idx + len(candidate) + 80
	if end > len(text) {
		end = len(text)
	}
	context := strings.ToLower(text[start:end])
	for _, marker := range []string{"dataset", "data set", "benchmark", "corpus", "suite", "数据集", "基准", "语料"} {
		if strings.Contains(context, marker) {
			return true
		}
	}
	return false
}

func datasetStopword(s string) bool {
	switch s {
	case "the", "this", "that", "figure", "table", "section", "appendix", "method", "methods", "result", "results", "transformer", "attention", "relu", "ln", "pre", "post", "nvidia", "gpu", "gpus", "cpu", "cpus", "cuda", "rtx", "tesla", "geforce", "amd", "intel", "v100", "a100", "h100":
		return true
	default:
		return semanticStopword(s)
	}
}

func semanticStopword(s string) bool {
	switch s {
	case "the", "and", "for", "with", "via", "from", "that", "this", "using", "based", "model", "models", "method", "methods", "results", "paper", "propose", "proposes", "image", "images", "generation", "generated", "approach", "task", "tasks", "how", "can", "what", "why", "when", "where", "whether", "pre", "post", "layer", "architecture", "nvidia", "gpu", "gpus", "cpu", "cpus", "cuda":
		return true
	default:
		return false
	}
}

func semanticNorm(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevSpace = false
		default:
			if !prevSpace && b.Len() > 0 {
				b.WriteRune(' ')
				prevSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func shortEvidence(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	rs := []rune(s)
	if len(rs) > 100 {
		return string(rs[:100]) + "..."
	}
	return s
}

func evidenceSegments(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var parts []string
	var b strings.Builder
	flush := func() {
		part := strings.TrimSpace(b.String())
		b.Reset()
		if len([]rune(part)) < 8 {
			return
		}
		parts = append(parts, part)
	}
	for _, r := range s {
		switch r {
		case '\n', '\r', '.', '。', ';', '；', '!', '！', '?', '？':
			flush()
		default:
			b.WriteRune(r)
		}
	}
	flush()
	if len(parts) == 0 {
		return []string{s}
	}
	if len(parts) > 12 {
		return parts[:12]
	}
	return parts
}

func saveSemanticRelations(ctx context.Context, owner, sourcePaperID string, matches []graphstore.SemanticMatch) error {
	relations := make([]model.PaperSemanticRelation, 0, len(matches))
	for _, match := range matches {
		relations = append(relations, model.PaperSemanticRelation{
			OwnerID:                    owner,
			SourcePaperID:              sourcePaperID,
			TargetPaperID:              match.TargetID,
			RelationType:               "SEMANTIC_SIMILAR",
			Score:                      match.Score,
			MatchedFields:              model.JSONStrings(match.MatchedFields),
			Summary:                    match.Summary,
			KeywordSimilarity:          match.KeywordSimilarity,
			ResearchQuestionSimilarity: match.ResearchQuestionSimilarity,
			MethodSimilarity:           match.MethodSimilarity,
			ExperimentSimilarity:       match.ExperimentSimilarity,
			InnovationSimilarity:       match.InnovationSimilarity,
			Model:                      match.Model,
		})
	}
	return paperdao.ReplaceSemanticRelations(ctx, owner, sourcePaperID, relations)
}

func profileOf(pg graphstore.PaperGraph) profileInput {
	return profileInput{
		Title:             strings.TrimSpace(pg.Title),
		Keywords:          nonEmpty(pg.Keywords),
		ResearchQuestions: nonEmpty(pg.ResearchQuestions),
		Methods:           strings.TrimSpace(pg.Methods),
		Experiments:       strings.TrimSpace(pg.Experiments),
	}
}

func nonEmpty(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return "未命名论文"
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

const semanticJudgePrompt = `你是科研论文知识图谱的语义关系判定器。请比较两篇论文，只依据输入的关键词、研究问题、方法、实验/数据集判断是否存在实质相似。

要求：
1. 识别同义、中英文翻译、缩写、表达长短不同但含义接近的内容。
2. 不要因为作者、机构、年份、通用大领域相同而判定相似。
3. 输入里的 candidate_evidence 是程序预提取的候选相似证据，请逐条检查。若证据合理，应尽量保留或改写到对应 similarity 字段；若证据只是表面同词或硬件环境词，请丢弃。
4. 不要输出笼统的“语义画像相似”。必须指出关键词、研究问题、方法、实验/数据集中的具体相似片段。
5. 实验/数据集相似只看数据集、benchmark、corpus、评测任务或实验协议；GPU、NVIDIA、CUDA、CPU、训练轮数、显存、服务器等硬件或运行环境不能作为实验/数据集相似依据。
6. 如果找不到具体片段，is_similar 必须为 false，matched_fields 为空数组，各 similarity 字段为空字符串。
7. 只输出 JSON，不要输出 Markdown。

JSON 格式：
{
  "is_similar": true,
  "score": 0.0,
  "matched_fields": ["关键词", "研究问题", "方法", "实验/数据集"],
  "summary": "一句话总结两篇论文为什么相似，必须基于具体字段",
  "keyword_similarity": "关键词层面的具体相似片段，没有则为空字符串",
  "research_question_similarity": "研究问题层面的具体相似片段，没有则为空字符串",
  "method_similarity": "方法层面的具体相似片段，没有则为空字符串",
  "experiment_similarity": "实验/数据集层面的具体相似片段，没有则为空字符串"
}`
