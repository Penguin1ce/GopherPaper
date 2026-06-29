package graph

import (
	"fmt"
	"strings"
)

type PaperSemanticProfile struct {
	Owner             string
	ID                string
	Title             string
	Keywords          []string
	ResearchQuestions []string
	Methods           string
	Experiments       string
	Innovations       []string
	Text              string
	Embedding         []float64
}

func BuildPaperSemanticProfile(p PaperGraph) PaperSemanticProfile {
	profile := PaperSemanticProfile{
		Owner:             p.Owner,
		ID:                p.ID,
		Title:             strings.TrimSpace(p.Title),
		Keywords:          cleanStringList(p.Keywords),
		ResearchQuestions: cleanStringList(p.ResearchQuestions),
		Methods:           strings.TrimSpace(p.Methods),
		Experiments:       strings.TrimSpace(p.Experiments),
		Innovations:       cleanStringList(p.Innovations),
		Embedding:         p.SemanticEmbedding,
	}
	profile.Text = SemanticProfileText(profile)
	return profile
}

func SemanticProfileText(p PaperSemanticProfile) string {
	var b strings.Builder
	writeText := func(label, value string, maxRunes int) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		fmt.Fprintf(&b, "%s: %s\n", label, truncateRunes(value, maxRunes))
	}
	writeList := func(label string, values []string, limit int) {
		values = cleanStringList(values)
		if limit > 0 && len(values) > limit {
			values = values[:limit]
		}
		if len(values) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s: %s\n", label, strings.Join(values, "; "))
	}

	writeText("论文名称", p.Title, 300)
	writeList("关键词", p.Keywords, 24)
	writeList("研究问题", p.ResearchQuestions, 10)
	writeText("方法", p.Methods, 1400)
	writeText("实验/数据集", p.Experiments, 1400)
	writeList("创新点", p.Innovations, 10)
	return strings.TrimSpace(b.String())
}

func cleanStringList(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = truncateRunes(strings.TrimSpace(value), 800)
		if value == "" {
			continue
		}
		norm := normalizeTitle(value)
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, value)
	}
	return out
}
