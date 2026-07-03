package graph

import (
	"context"
	"fmt"
	"strings"
)

type Stats struct {
	Papers    int `json:"papers"`
	Authors   int `json:"authors"`
	Keywords  int `json:"keywords"`
	Citations int `json:"citations"`
	MinYear   int `json:"min_year"`
	MaxYear   int `json:"max_year"`
}

type YearCount struct {
	Year  int `json:"year"`
	Count int `json:"count"`
}

type KeywordYearCount struct {
	Keyword string `json:"keyword"`
	Year    int    `json:"year"`
	Count   int    `json:"count"`
}

type NameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type EntityNode struct {
	ID      string            `json:"id"`
	Type    string            `json:"type"`
	Label   string            `json:"label"`
	Details map[string]string `json:"details,omitempty"`
}

type EntityEdge struct {
	ID      string            `json:"id"`
	Source  string            `json:"source"`
	Target  string            `json:"target"`
	Type    string            `json:"type"`
	Label   string            `json:"label"`
	Details map[string]string `json:"details,omitempty"`
}

type EntityGraph struct {
	Nodes []EntityNode `json:"nodes"`
	Edges []EntityEdge `json:"edges"`
}

type Related struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Year  int      `json:"year"`
	Score int      `json:"score"`
	Vias  []string `json:"vias"`
}

func Overview(ctx context.Context, owner string) (Stats, error) {
	const cypher = `
CALL { MATCH (p:Paper {owner:$owner}) RETURN count(p) AS papers }
CALL { MATCH (a:Author {owner:$owner}) RETURN count(a) AS authors }
CALL { MATCH (k:Keyword {owner:$owner}) RETURN count(k) AS keywords }
CALL { MATCH (:Paper {owner:$owner})-[c:CITES]->() RETURN count(c) AS citations }
CALL { MATCH (y:Paper {owner:$owner}) WHERE y.year > 0 RETURN min(y.year) AS minYear, max(y.year) AS maxYear }
RETURN papers, authors, keywords, citations, minYear, maxYear`
	res, err := exec(ctx, cypher, map[string]any{"owner": owner})
	if err != nil {
		return Stats{}, err
	}
	var s Stats
	if len(res.Records) > 0 {
		r := res.Records[0]
		s = Stats{
			Papers:    asInt(r, "papers"),
			Authors:   asInt(r, "authors"),
			Keywords:  asInt(r, "keywords"),
			Citations: asInt(r, "citations"),
			MinYear:   asInt(r, "minYear"),
			MaxYear:   asInt(r, "maxYear"),
		}
	}
	return s, nil
}

func TrendByYear(ctx context.Context, owner string) ([]YearCount, error) {
	const cypher = `
MATCH (p:Paper {owner:$owner}) WHERE p.year > 0
RETURN p.year AS year, count(p) AS count
ORDER BY year`
	res, err := exec(ctx, cypher, map[string]any{"owner": owner})
	if err != nil {
		return nil, err
	}
	out := make([]YearCount, 0, len(res.Records))
	for _, r := range res.Records {
		out = append(out, YearCount{Year: asInt(r, "year"), Count: asInt(r, "count")})
	}
	return out, nil
}

func KeywordTrend(ctx context.Context, owner string, topN int) ([]KeywordYearCount, error) {
	if topN <= 0 {
		topN = 10
	}
	const cypher = `
MATCH (p:Paper {owner:$owner})-[:HAS_KEYWORD]->(k:Keyword) WHERE p.year > 0
WITH k, count(*) AS total ORDER BY total DESC LIMIT $topN
MATCH (p2:Paper {owner:$owner})-[:HAS_KEYWORD]->(k) WHERE p2.year > 0
RETURN k.name AS keyword, p2.year AS year, count(*) AS count
ORDER BY keyword, year`
	res, err := exec(ctx, cypher, map[string]any{"owner": owner, "topN": topN})
	if err != nil {
		return nil, err
	}
	out := make([]KeywordYearCount, 0, len(res.Records))
	for _, r := range res.Records {
		out = append(out, KeywordYearCount{
			Keyword: asStr(r, "keyword"),
			Year:    asInt(r, "year"),
			Count:   asInt(r, "count"),
		})
	}
	return out, nil
}

func TopKeywords(ctx context.Context, owner string, topN int) ([]NameCount, error) {
	if topN <= 0 {
		topN = 20
	}
	const cypher = `
MATCH (:Paper {owner:$owner})-[:HAS_KEYWORD]->(k:Keyword)
RETURN k.name AS name, count(*) AS count
ORDER BY count DESC, name LIMIT $topN`
	res, err := exec(ctx, cypher, map[string]any{"owner": owner, "topN": topN})
	if err != nil {
		return nil, err
	}
	out := make([]NameCount, 0, len(res.Records))
	for _, r := range res.Records {
		out = append(out, NameCount{Name: asStr(r, "name"), Count: asInt(r, "count")})
	}
	return out, nil
}

func OverviewEntityGraph(ctx context.Context, owner string) (EntityGraph, error) {
	const papersCypher = `
MATCH (p:Paper {owner:$owner})
RETURN p.id AS id, coalesce(p.title, p.id) AS title, p.year AS year, p.venue AS venue
ORDER BY title`
	paperRes, err := exec(ctx, papersCypher, map[string]any{"owner": owner})
	if err != nil {
		return EntityGraph{}, err
	}
	g := emptyEntityGraph()
	seenNodes := map[string]bool{}
	seenEdges := map[string]bool{}

	for _, r := range paperRes.Records {
		id := asStr(r, "id")
		if id == "" {
			continue
		}
		nodeID := "paper:" + id
		if seenNodes[nodeID] {
			continue
		}
		g.Nodes = append(g.Nodes, EntityNode{
			ID:    nodeID,
			Type:  "paper",
			Label: asStr(r, "title"),
			Details: map[string]string{
				"paper_id": id,
				"year":     intString(asInt(r, "year")),
				"venue":    asStr(r, "venue"),
			},
		})
		seenNodes[nodeID] = true
	}

	const linksCypher = `
MATCH (p:Paper {owner:$owner})-[:AUTHORED_BY]->(n:Author)
WITH n, collect(DISTINCT p) AS ps WHERE size(ps) > 1
UNWIND ps AS p
RETURN p.id AS paperID, 'Author:' + coalesce(n.norm, toString(id(n))) AS nodeID, 'Author' AS nodeType, coalesce(n.name, '') AS nodeLabel, 'AUTHORED_BY' AS relType
UNION
MATCH (p:Paper {owner:$owner})-[:HAS_KEYWORD]->(n:Keyword)
WITH n, collect(DISTINCT p) AS ps WHERE size(ps) > 1
UNWIND ps AS p
RETURN p.id AS paperID, 'Keyword:' + coalesce(n.norm, toString(id(n))) AS nodeID, 'Keyword' AS nodeType, coalesce(n.name, '') AS nodeLabel, 'HAS_KEYWORD' AS relType
UNION
MATCH (p:Paper {owner:$owner})-[:FROM_AFFILIATION]->(n:Affiliation)
WITH n, collect(DISTINCT p) AS ps WHERE size(ps) > 1
UNWIND ps AS p
RETURN p.id AS paperID, 'Affiliation:' + coalesce(n.norm, toString(id(n))) AS nodeID, 'Affiliation' AS nodeType, coalesce(n.name, '') AS nodeLabel, 'FROM_AFFILIATION' AS relType
UNION
MATCH (p:Paper {owner:$owner})-[:CITES]->(n:Reference)
WITH n, collect(DISTINCT p) AS ps WHERE size(ps) > 1
UNWIND ps AS p
RETURN p.id AS paperID, 'Reference:' + coalesce(n.key, toString(id(n))) AS nodeID, 'Reference' AS nodeType, coalesce(n.raw, '') AS nodeLabel, 'CITES' AS relType`
	linkRes, err := exec(ctx, linksCypher, map[string]any{"owner": owner})
	if err != nil {
		return EntityGraph{}, err
	}
	for _, r := range linkRes.Records {
		paperID := asStr(r, "paperID")
		nodeKey := asStr(r, "nodeID")
		label := asStr(r, "nodeLabel")
		relType := asStr(r, "relType")
		if paperID == "" || nodeKey == "" || label == "" || relType == "" {
			continue
		}
		paperNodeID := "paper:" + paperID
		entityNodeID := "entity:" + nodeKey
		nodeType := graphNodeType(asStr(r, "nodeType"))
		if !seenNodes[entityNodeID] {
			g.Nodes = append(g.Nodes, EntityNode{
				ID:    entityNodeID,
				Type:  nodeType,
				Label: label,
				Details: map[string]string{
					"type": nodeType,
				},
			})
			seenNodes[entityNodeID] = true
		}
		edgeID := paperNodeID + ":" + relType + ":" + entityNodeID
		if seenEdges[edgeID] {
			continue
		}
		g.Edges = append(g.Edges, EntityEdge{
			ID:     edgeID,
			Source: paperNodeID,
			Target: entityNodeID,
			Type:   relType,
			Label:  relationLabel(relType),
		})
		seenEdges[edgeID] = true
	}
	if err := appendCitationOverviewEdges(ctx, owner, &g, seenEdges); err != nil {
		return EntityGraph{}, err
	}
	if err := appendSemanticOverviewEdges(ctx, owner, &g, seenEdges); err != nil {
		return EntityGraph{}, err
	}
	return g, nil
}

func appendCitationOverviewEdges(ctx context.Context, owner string, g *EntityGraph, seenEdges map[string]bool) error {
	const cypher = `
MATCH (p:Paper {owner:$owner})-[c:CITES]->(q:Paper {owner:$owner})
RETURN p.id AS sourceID, q.id AS targetID, type(c) AS relType
ORDER BY sourceID, targetID`
	res, err := exec(ctx, cypher, map[string]any{"owner": owner})
	if err != nil {
		return err
	}
	for _, r := range res.Records {
		sourceID := asStr(r, "sourceID")
		targetID := asStr(r, "targetID")
		relType := asStr(r, "relType")
		if sourceID == "" || targetID == "" || relType == "" {
			continue
		}
		edgeID := "paper:" + sourceID + ":" + relType + ":paper:" + targetID
		if seenEdges[edgeID] {
			continue
		}
		g.Edges = append(g.Edges, EntityEdge{
			ID:     edgeID,
			Source: "paper:" + sourceID,
			Target: "paper:" + targetID,
			Type:   relType,
			Label:  relationLabel(relType),
		})
		seenEdges[edgeID] = true
	}
	return nil
}

func appendSemanticOverviewEdges(ctx context.Context, owner string, g *EntityGraph, seenEdges map[string]bool) error {
	const cypher = `
MATCH (p:Paper {owner:$owner})-[s:SEMANTIC_SIMILAR]-(q:Paper {owner:$owner})
WHERE p.id < q.id
RETURN p.id AS sourceID,
       q.id AS targetID,
       s.score AS score,
       coalesce(s.matched_fields, []) AS matchedFields,
       coalesce(s.summary, '') AS summary,
       coalesce(s.keyword_similarity, '') AS keywordSimilarity,
       coalesce(s.research_question_similarity, '') AS researchQuestionSimilarity,
       coalesce(s.method_similarity, '') AS methodSimilarity,
       coalesce(s.experiment_similarity, '') AS experimentSimilarity,
       coalesce(s.innovation_similarity, '') AS innovationSimilarity,
       coalesce(s.model, '') AS model
ORDER BY score DESC`
	res, err := exec(ctx, cypher, map[string]any{"owner": owner})
	if err != nil {
		return err
	}
	for _, r := range res.Records {
		sourceID := asStr(r, "sourceID")
		targetID := asStr(r, "targetID")
		if sourceID == "" || targetID == "" {
			continue
		}
		edgeID := "paper:" + sourceID + ":SEMANTIC_SIMILAR:paper:" + targetID
		if seenEdges[edgeID] {
			continue
		}
		details := map[string]string{
			"相似摘要":   asStr(r, "summary"),
			"相似字段":   joinNonEmpty(asStrSlice(r, "matchedFields"), "、"),
			"相似分数":   fmt.Sprintf("%.3f", asFloat(r, "score")),
			"关键词":    asStr(r, "keywordSimilarity"),
			"研究问题":   asStr(r, "researchQuestionSimilarity"),
			"方法":     asStr(r, "methodSimilarity"),
			"实验/数据集": asStr(r, "experimentSimilarity"),
			"创新点":    asStr(r, "innovationSimilarity"),
			"判定模型":   asStr(r, "model"),
		}
		g.Edges = append(g.Edges, EntityEdge{
			ID:      edgeID,
			Source:  "paper:" + sourceID,
			Target:  "paper:" + targetID,
			Type:    "SEMANTIC_SIMILAR",
			Label:   "SEMANTIC_SIMILAR",
			Details: compactDetails(details),
		})
		seenEdges[edgeID] = true
	}
	return nil
}

func PaperEntityGraph(ctx context.Context, owner, paperID string) (EntityGraph, error) {
	const cypher = `
MATCH (p:Paper {owner:$owner, id:$id})
CALL {
  WITH p
  OPTIONAL MATCH (p)-[r:AUTHORED_BY|HAS_KEYWORD|FROM_AFFILIATION|PUBLISHED_IN|HAS_RESEARCH_QUESTION|USES_METHOD|HAS_EXPERIMENT|HAS_RESULT|HAS_INNOVATION|HAS_LIMITATION|HAS_FUTURE_WORK|CITES]->(n)
  WITH collect(CASE WHEN n IS NULL THEN null ELSE {
    node_id: CASE WHEN n:Paper THEN 'Paper:' + n.id ELSE head(labels(n)) + ':' + coalesce(n.norm, n.key, elementId(n)) END,
    node_type: head(labels(n)),
    node_label: coalesce(n.name, n.raw, n.title, ''),
    rel_type: type(r)
  } END) AS rows
  RETURN [x IN rows WHERE x IS NOT NULL] AS items
}
RETURN p.id AS paperID, coalesce(p.title, p.id) AS title, p.year AS year, p.venue AS venue, items`
	res, err := exec(ctx, cypher, map[string]any{"owner": owner, "id": paperID})
	if err != nil {
		return EntityGraph{}, err
	}
	if len(res.Records) == 0 {
		return emptyEntityGraph(), nil
	}

	r := res.Records[0]
	paperID = asStr(r, "paperID")
	paperNodeID := "paper:" + paperID
	g := EntityGraph{
		Nodes: []EntityNode{{
			ID:    paperNodeID,
			Type:  "paper",
			Label: asStr(r, "title"),
			Details: map[string]string{
				"paper_id": paperID,
				"year":     intString(asInt(r, "year")),
				"venue":    asStr(r, "venue"),
			},
		}},
		Edges: []EntityEdge{},
	}

	items, _ := r.Get("items")
	rawItems, _ := items.([]any)
	seenNodes := map[string]bool{paperNodeID: true}
	seenEdges := map[string]bool{}
	for _, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		rawNodeID := stringFromMap(item, "node_id")
		label := stringFromMap(item, "node_label")
		relType := stringFromMap(item, "rel_type")
		nodeType := graphNodeType(stringFromMap(item, "node_type"))
		nodeID := "entity:" + rawNodeID
		if nodeType == "paper" {
			citedPaperID := strings.TrimPrefix(rawNodeID, "Paper:")
			nodeID = "paper:" + citedPaperID
		}
		if rawNodeID == "" || nodeID == "entity:" || nodeID == "paper:" || label == "" || relType == "" {
			continue
		}
		if !seenNodes[nodeID] {
			g.Nodes = append(g.Nodes, EntityNode{
				ID:    nodeID,
				Type:  nodeType,
				Label: label,
				Details: map[string]string{
					"type": nodeType,
				},
			})
			seenNodes[nodeID] = true
		}
		edgeID := paperNodeID + ":" + relType + ":" + nodeID
		if seenEdges[edgeID] {
			continue
		}
		g.Edges = append(g.Edges, EntityEdge{
			ID:     edgeID,
			Source: paperNodeID,
			Target: nodeID,
			Type:   relType,
			Label:  relationLabel(relType),
		})
		seenEdges[edgeID] = true
	}
	return g, nil
}

func emptyEntityGraph() EntityGraph {
	return EntityGraph{
		Nodes: []EntityNode{},
		Edges: []EntityEdge{},
	}
}

func RelatedPapers(ctx context.Context, owner, paperID string, limit int) ([]Related, error) {
	if limit <= 0 {
		limit = 10
	}
	const cypher = `
MATCH (p:Paper {owner:$owner, id:$id})
CALL {
  WITH p MATCH (p)-[:AUTHORED_BY]->(:Author)<-[:AUTHORED_BY]-(q:Paper) WHERE q.id <> p.id RETURN q, count(*) AS w, 'author' AS via
  UNION
  WITH p MATCH (p)-[:HAS_KEYWORD]->(:Keyword)<-[:HAS_KEYWORD]-(q:Paper) WHERE q.id <> p.id RETURN q, count(*) AS w, 'keyword' AS via
  UNION
  WITH p MATCH (p)-[:CITES]->(:Reference)<-[:CITES]-(q:Paper) WHERE q.id <> p.id RETURN q, count(*) AS w, 'cocitation' AS via
  UNION
  WITH p MATCH (p)-[:CITES]->(q:Paper) WHERE q.id <> p.id RETURN q, 1 AS w, 'cites' AS via
  UNION
  WITH p MATCH (p)-[s:SIMILAR_TO]-(q:Paper) WHERE q.id <> p.id WITH q, max(s.score) AS sim RETURN q, toInteger(round(sim * 8)) AS w, 'similar' AS via
}
WITH q, sum(w) AS score, collect(DISTINCT via) AS vias
RETURN q.id AS id, q.title AS title, q.year AS year, score, vias
ORDER BY score DESC, q.title LIMIT $limit`
	res, err := exec(ctx, cypher, map[string]any{"owner": owner, "id": paperID, "limit": limit})
	if err != nil {
		return nil, err
	}
	out := make([]Related, 0, len(res.Records))
	for _, r := range res.Records {
		out = append(out, Related{
			ID:    asStr(r, "id"),
			Title: asStr(r, "title"),
			Year:  asInt(r, "year"),
			Score: asInt(r, "score"),
			Vias:  asStrSlice(r, "vias"),
		})
	}
	return out, nil
}

func stringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func asAnySlice(v any) []any {
	if items, ok := v.([]any); ok {
		return items
	}
	return nil
}

func anyIntString(v any) string {
	if n, ok := v.(int64); ok && n > 0 {
		return fmt.Sprintf("%d", n)
	}
	if n, ok := v.(int); ok && n > 0 {
		return fmt.Sprintf("%d", n)
	}
	return ""
}

func intString(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", n)
}

func asFloat(rec recordGetter, key string) float64 {
	v, ok := rec.Get(key)
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int64:
		return float64(n)
	case int:
		return float64(n)
	default:
		return 0
	}
}

type recordGetter interface {
	Get(string) (any, bool)
}

func compactDetails(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		if v != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func joinNonEmpty(values []string, sep string) string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, sep)
}

func graphNodeType(label string) string {
	switch label {
	case "Paper":
		return "paper"
	case "Author":
		return "author"
	case "Affiliation":
		return "affiliation"
	case "Keyword":
		return "keyword"
	case "Venue":
		return "venue"
	case "ResearchQuestion":
		return "research_question"
	case "Method":
		return "method"
	case "Experiment":
		return "experiment"
	case "Result":
		return "result"
	case "Innovation":
		return "innovation"
	case "Limitation":
		return "limitation"
	case "FutureWork":
		return "future_work"
	case "Reference":
		return "reference"
	default:
		return "entity"
	}
}

func relationLabel(rel string) string {
	return rel
}
