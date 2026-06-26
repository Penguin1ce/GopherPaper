package graph

import "context"

// Stats 是某用户图谱的总览统计。
type Stats struct {
	Papers    int `json:"papers"`
	Authors   int `json:"authors"`
	Keywords  int `json:"keywords"`
	Citations int `json:"citations"`
	MinYear   int `json:"min_year"`
	MaxYear   int `json:"max_year"`
}

// YearCount 是某一年的论文数,用于时间趋势。
type YearCount struct {
	Year  int `json:"year"`
	Count int `json:"count"`
}

// KeywordYearCount 是某关键词在某年的出现次数,用于关键词热度演化。
type KeywordYearCount struct {
	Keyword string `json:"keyword"`
	Year    int    `json:"year"`
	Count   int    `json:"count"`
}

// NameCount 是名称与计数,用于 Top 关键词/作者。
type NameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Related 是与某篇论文相关的论文,score 为各关系加权,vias 标关系类型。
type Related struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Year  int      `json:"year"`
	Score int      `json:"score"`
	Vias  []string `json:"vias"` // author / keyword / cocitation / cites / similar
}

// Overview 汇总某用户图谱规模:节点/边计数与年份跨度。
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

// TrendByYear 返回每年论文数,按年份升序,只统计抽到年份的论文。
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

// KeywordTrend 返回 Top N 热门关键词在各年份的出现次数,用于关键词热度随时间演化。
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

// TopKeywords 返回出现最多的 Top N 关键词。
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

// RelatedPapers 返回与给定论文相关的论文,关系来自共享作者/共享关键词/共被引/直接引用/语义相似,
// 按加权出现次数排序。owner 隔离由节点的 owner 属性天然保证。
// 语义相似边把字面关键词难重合的同领域论文连上,权重按 cosine 相似度放大成整数(round(sim*5))。
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
