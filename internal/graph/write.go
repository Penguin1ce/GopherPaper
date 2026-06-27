package graph

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"GopherPaper/pkg/constant"
)

// PaperGraph is the write model for one paper-centered graph.
type PaperGraph struct {
	Owner             string
	ID                string
	Title             string
	Year              int
	Venue             string
	Authors           []string
	Keywords          []string
	Affiliations      []string
	ResearchQuestions []string
	Methods           string
	Experiments       string
	Results           string
	Innovations       []string
	Limitations       []string
	FutureWork        []string
	Embedding         []float64
}

const upsertPaperBaseCypher = `
MERGE (p:Paper {owner:$owner, id:$id})
SET p.title=$title,
    p.norm_title=$normTitle,
    p.year=$year,
    p.venue=$venue,
    p.embedding=$embedding,
    p.updated_at=timestamp()
WITH p
OPTIONAL MATCH (p)-[r:AUTHORED_BY|HAS_KEYWORD|FROM_AFFILIATION|PUBLISHED_IN|HAS_RESEARCH_QUESTION|USES_METHOD|HAS_EXPERIMENT|HAS_RESULT|HAS_INNOVATION|HAS_LIMITATION|HAS_FUTURE_WORK]->()
DELETE r
RETURN p.id AS id`

type entityWriteSpec struct {
	param string
	label string
	rel   string
}

var metadataEntitySpecs = []entityWriteSpec{
	{param: "authors", label: "Author", rel: "AUTHORED_BY"},
	{param: "keywords", label: "Keyword", rel: "HAS_KEYWORD"},
	{param: "affiliations", label: "Affiliation", rel: "FROM_AFFILIATION"},
	{param: "venues", label: "Venue", rel: "PUBLISHED_IN"},
	{param: "researchQuestions", label: "ResearchQuestion", rel: "HAS_RESEARCH_QUESTION"},
	{param: "methods", label: "Method", rel: "USES_METHOD"},
	{param: "experiments", label: "Experiment", rel: "HAS_EXPERIMENT"},
	{param: "results", label: "Result", rel: "HAS_RESULT"},
	{param: "innovations", label: "Innovation", rel: "HAS_INNOVATION"},
	{param: "limitations", label: "Limitation", rel: "HAS_LIMITATION"},
	{param: "futureWork", label: "FutureWork", rel: "HAS_FUTURE_WORK"},
}

// UpsertPaper writes one paper and its metadata graph, then refreshes outgoing
// semantic similarity edges when an embedding is available.
func UpsertPaper(ctx context.Context, p PaperGraph) error {
	if err := UpsertPaperMetadata(ctx, p); err != nil {
		return err
	}
	return relinkSimilar(ctx, p.Owner, p.ID)
}

// UpsertPaperMetadata writes only the paper-centered metadata graph. It is used
// for repair/rebuild flows that read already parsed metadata from MySQL.
func UpsertPaperMetadata(ctx context.Context, p PaperGraph) error {
	venues := []string{}
	if v := strings.TrimSpace(p.Venue); v != "" {
		venues = []string{v}
	}
	embedding := p.Embedding
	if embedding == nil {
		embedding = []float64{}
	}
	params := map[string]any{
		"owner":             p.Owner,
		"id":                p.ID,
		"title":             strings.TrimSpace(p.Title),
		"normTitle":         normalizeTitle(p.Title),
		"year":              p.Year,
		"venue":             strings.TrimSpace(p.Venue),
		"embedding":         embedding,
		"authors":           cleanTerms(p.Authors),
		"keywords":          cleanTerms(p.Keywords),
		"affiliations":      cleanTerms(p.Affiliations),
		"venues":            cleanTerms(venues),
		"researchQuestions": cleanTerms(p.ResearchQuestions),
		"methods":           cleanScalarTerm(p.Methods),
		"experiments":       cleanScalarTerm(p.Experiments),
		"results":           cleanScalarTerm(p.Results),
		"innovations":       cleanTerms(p.Innovations),
		"limitations":       cleanTerms(p.Limitations),
		"futureWork":        cleanTerms(p.FutureWork),
	}
	if _, err := exec(ctx, upsertPaperBaseCypher, params); err != nil {
		return fmt.Errorf("upsert paper node: %w", err)
	}
	for _, spec := range metadataEntitySpecs {
		terms, _ := params[spec.param].([]map[string]any)
		if len(terms) == 0 {
			continue
		}
		if err := upsertMetadataTerms(ctx, p.Owner, p.ID, spec, terms); err != nil {
			return err
		}
	}
	return CleanupOrphans(ctx, p.Owner)
}

func upsertMetadataTerms(ctx context.Context, owner, paperID string, spec entityWriteSpec, terms []map[string]any) error {
	cypher := fmt.Sprintf(`
MATCH (p:Paper {owner:$owner, id:$id})
UNWIND $terms AS t
MERGE (n:%s {owner:$owner, norm:t.norm})
ON CREATE SET n.name=t.name
SET n.name=coalesce(n.name, t.name)
MERGE (p)-[:%s]->(n)
RETURN count(n) AS count`, spec.label, spec.rel)
	if _, err := exec(ctx, cypher, map[string]any{
		"owner": owner,
		"id":    paperID,
		"terms": terms,
	}); err != nil {
		return fmt.Errorf("upsert %s terms: %w", spec.label, err)
	}
	return nil
}

const relinkSimilarDeleteCypher = `
MATCH (p:Paper {owner:$owner, id:$id})-[s:SIMILAR_TO]->() DELETE s`

const relinkSimilarCypher = `
MATCH (p:Paper {owner:$owner, id:$id})
WHERE p.embedding IS NOT NULL AND size(p.embedding) > 0
MATCH (q:Paper {owner:$owner})
WHERE q.id <> p.id AND q.embedding IS NOT NULL AND size(q.embedding) > 0
WITH p, q, vector.similarity.cosine(p.embedding, q.embedding) AS sim
WHERE sim >= $threshold
WITH p, q, sim ORDER BY sim DESC LIMIT $topK
MERGE (p)-[s:SIMILAR_TO]->(q) SET s.score = sim, s.updated_at = timestamp()`

func relinkSimilar(ctx context.Context, owner, id string) error {
	if _, err := exec(ctx, relinkSimilarDeleteCypher, map[string]any{"owner": owner, "id": id}); err != nil {
		return err
	}
	_, err := exec(ctx, relinkSimilarCypher, map[string]any{
		"owner":     owner,
		"id":        id,
		"threshold": constant.GraphSimilarThreshold,
		"topK":      constant.GraphSimilarTopK,
	})
	return err
}

const deleteCitationsCypher = `
MATCH (p:Paper {owner:$owner, id:$id})-[c:CITES]->()
DELETE c`

const upsertReferenceCitationsCypher = `
MATCH (p:Paper {owner:$owner, id:$id})
UNWIND $refs AS ref
MERGE (r:Reference {owner:$owner, key:ref.key})
ON CREATE SET r.raw=ref.raw
SET r.raw=coalesce(r.raw, ref.raw)
MERGE (p)-[:CITES]->(r)
RETURN count(r) AS count`

const upsertPaperCitationsCypher = `
MATCH (p:Paper {owner:$owner, id:$id})
UNWIND $refNorms AS rn
MATCH (q:Paper {owner:$owner})
WHERE q.id <> p.id AND q.norm_title <> '' AND rn CONTAINS q.norm_title
MERGE (p)-[:CITES]->(q)
RETURN count(q) AS count`

// UpsertCitations rewrites CITES edges for one paper.
func UpsertCitations(ctx context.Context, owner, paperID string, refs []string) error {
	refMaps := make([]map[string]any, 0, len(refs))
	norms := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, raw := range refs {
		raw = strings.TrimSpace(raw)
		norm := normalizeTitle(raw)
		if len([]rune(norm)) < 10 || seen[norm] {
			continue
		}
		seen[norm] = true
		if len([]rune(norm)) > 300 {
			norm = string([]rune(norm)[:300])
		}
		refMaps = append(refMaps, map[string]any{"key": norm, "raw": raw})
		norms = append(norms, norm)
	}
	base := map[string]any{"owner": owner, "id": paperID}
	if _, err := exec(ctx, deleteCitationsCypher, base); err != nil {
		return err
	}
	if len(refMaps) > 0 {
		params := map[string]any{"owner": owner, "id": paperID, "refs": refMaps}
		if _, err := exec(ctx, upsertReferenceCitationsCypher, params); err != nil {
			return err
		}
	}
	if len(norms) > 0 {
		params := map[string]any{"owner": owner, "id": paperID, "refNorms": norms}
		if _, err := exec(ctx, upsertPaperCitationsCypher, params); err != nil {
			return err
		}
	}
	return nil
}

func DeletePaper(ctx context.Context, owner, paperID string) error {
	if _, err := exec(ctx, `MATCH (p:Paper {owner:$owner, id:$id}) DETACH DELETE p`,
		map[string]any{"owner": owner, "id": paperID}); err != nil {
		return err
	}
	return CleanupOrphans(ctx, owner)
}

const cleanupOrphansCypher = `
MATCH (n)
WHERE n.owner=$owner AND (n:Author OR n:Keyword OR n:Affiliation OR n:Venue OR n:ResearchQuestion OR n:Method OR n:Experiment OR n:Result OR n:Innovation OR n:Limitation OR n:FutureWork OR n:Reference) AND NOT (n)--()
DELETE n`

func CleanupOrphans(ctx context.Context, owner string) error {
	_, err := exec(ctx, cleanupOrphansCypher, map[string]any{"owner": owner})
	return err
}

func cleanTerms(in []string) []map[string]any {
	seen := map[string]bool{}
	out := make([]map[string]any, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		s = truncateRunes(s, 800)
		norm := truncateRunes(normalizeTitle(s), 300)
		if s == "" || norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, map[string]any{"name": s, "norm": norm})
	}
	return out
}

func cleanScalarTerm(s string) []map[string]any {
	return cleanTerms([]string{s})
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max])
}

func normalizeTitle(s string) string {
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
