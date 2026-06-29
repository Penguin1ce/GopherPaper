package graphsemantic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"GopherPaper/internal/ai/agentrt"
	paperdao "GopherPaper/internal/dao/paper"
	graphstore "GopherPaper/internal/graph"
	"GopherPaper/internal/knowledge"
	"GopherPaper/internal/model"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

const (
	semanticCandidateTopK = 8
	semanticAcceptScore   = 0.52
	semanticJudgeModel    = "agentrt-chat"
)

type profileInput struct {
	Title             string   `json:"title"`
	Keywords          []string `json:"keywords"`
	ResearchQuestions []string `json:"research_questions"`
	Methods           string   `json:"methods"`
	Experiments       string   `json:"experiments"`
	Innovations       []string `json:"innovations"`
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
	InnovationSimilarity       string   `json:"innovation_similarity"`
}

func PreparePaperGraph(ctx context.Context, pg graphstore.PaperGraph) graphstore.PaperGraph {
	profile := graphstore.BuildPaperSemanticProfile(pg)
	pg.SemanticProfile = profile.Text
	if pg.SemanticProfile == "" {
		return pg
	}
	vec, err := knowledge.Embed(ctx, pg.SemanticProfile)
	if err != nil {
		zlog.Error("论文语义画像向量化失败，跳过语义相似召回", "paper_id", pg.ID, "err", err)
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
	candidates, err := graphstore.SemanticCandidates(ctx, owner, paperID, semanticCandidateTopK, constant.GraphSimilarThreshold)
	if err != nil {
		return err
	}
	matches := make([]graphstore.SemanticMatch, 0, len(candidates))
	for _, candidate := range candidates {
		target, err := paperGraphFromMeta(ctx, owner, candidate.ID)
		if err != nil {
			zlog.Error("读取语义候选论文元信息失败，跳过", "paper_id", paperID, "target_id", candidate.ID, "err", err)
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
	return saveSemanticRelations(ctx, owner, paperID, matches)
}

func paperGraphFromMeta(ctx context.Context, owner, paperID string) (graphstore.PaperGraph, error) {
	p, err := paperdao.Get(ctx, paperID)
	if err != nil {
		return graphstore.PaperGraph{}, err
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
	in := map[string]profileInput{
		"paper_a": profileOf(source),
		"paper_b": profileOf(target),
	}
	payload, _ := json.MarshalIndent(in, "", "  ")
	content, err := agentrt.Generate(ctx, semanticJudgePrompt, nil, string(payload))
	if err != nil {
		zlog.Error("论文语义关系大模型精判失败，使用向量召回降级", "paper_id", source.ID, "target_id", target.ID, "err", err)
		return heuristicMatch(target.ID, recallScore, source, target)
	}
	var out judgeOutput
	if err := json.Unmarshal([]byte(extractJSON(content)), &out); err != nil {
		zlog.Error("论文语义关系精判 JSON 解析失败，使用向量召回降级", "paper_id", source.ID, "target_id", target.ID, "err", err)
		return heuristicMatch(target.ID, recallScore, source, target)
	}
	score := out.Score
	if score <= 0 {
		score = recallScore
	}
	if !out.IsSimilar || score < semanticAcceptScore || len(nonEmpty(out.MatchedFields)) == 0 {
		return graphstore.SemanticMatch{}, false
	}
	return graphstore.SemanticMatch{
		TargetID:                   target.ID,
		Score:                      score,
		MatchedFields:              nonEmpty(out.MatchedFields),
		Summary:                    strings.TrimSpace(out.Summary),
		KeywordSimilarity:          strings.TrimSpace(out.KeywordSimilarity),
		ResearchQuestionSimilarity: strings.TrimSpace(out.ResearchQuestionSimilarity),
		MethodSimilarity:           strings.TrimSpace(out.MethodSimilarity),
		ExperimentSimilarity:       strings.TrimSpace(out.ExperimentSimilarity),
		InnovationSimilarity:       strings.TrimSpace(out.InnovationSimilarity),
		Model:                      semanticJudgeModel,
	}, true
}

func heuristicMatch(targetID string, recallScore float64, source, target graphstore.PaperGraph) (graphstore.SemanticMatch, bool) {
	if recallScore < semanticAcceptScore {
		return graphstore.SemanticMatch{}, false
	}
	fields := []string{}
	if overlap(source.Keywords, target.Keywords) != "" {
		fields = append(fields, "关键词")
	}
	if recallScore >= 0.7 {
		fields = append(fields, "研究问题", "方法")
	}
	if len(fields) == 0 {
		fields = append(fields, "语义画像")
	}
	return graphstore.SemanticMatch{
		TargetID:      targetID,
		Score:         recallScore,
		MatchedFields: nonEmpty(fields),
		Summary:       fmt.Sprintf("两篇论文的语义画像向量相似度为 %.3f，建议作为相关论文查看。", recallScore),
		KeywordSimilarity: func() string {
			return overlap(source.Keywords, target.Keywords)
		}(),
		Model: "embedding-fallback",
	}, true
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
		Innovations:       nonEmpty(pg.Innovations),
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

func overlap(a, b []string) string {
	left := map[string]string{}
	for _, value := range nonEmpty(a) {
		left[strings.ToLower(value)] = value
	}
	matches := []string{}
	for _, value := range nonEmpty(b) {
		if original, ok := left[strings.ToLower(value)]; ok {
			matches = append(matches, original)
		}
	}
	return strings.Join(matches, "、")
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

const semanticJudgePrompt = `你是科研论文知识图谱的语义关系判定器。请比较两篇论文，只依据输入的关键词、研究问题、方法、实验/数据集、创新点判断是否存在实质相似。

要求：
1. 识别同义、中英文翻译、缩写、表达长短不同但含义接近的内容。
2. 不要因为作者、机构、年份相同而判定相似。
3. 只有在关键词、研究问题、方法、实验/数据集、创新点中至少一个字段有清晰相似信息时，is_similar 才能为 true。
4. 只输出 JSON，不要输出 Markdown。

JSON 格式：
{
  "is_similar": true,
  "score": 0.0,
  "matched_fields": ["关键词", "研究问题", "方法", "实验/数据集", "创新点"],
  "summary": "一句话总结两篇论文为什么相似",
  "keyword_similarity": "关键词层面的相似信息，没有则为空字符串",
  "research_question_similarity": "研究问题层面的相似信息，没有则为空字符串",
  "method_similarity": "方法层面的相似信息，没有则为空字符串",
  "experiment_similarity": "实验/数据集层面的相似信息，没有则为空字符串",
  "innovation_similarity": "创新点层面的相似信息，没有则为空字符串"
}`
