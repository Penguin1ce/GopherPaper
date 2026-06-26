package graph

import (
	"context"
	"strings"
	"unicode"

	"GopherPaper/pkg/constant"
)

// PaperGraph 是一篇论文写入图谱的入参,由 worker 从结构化抽取结果组装。
// 空字段(如无 venue/无作者)会被本包过滤,不会建出空节点。
// Embedding 为论文标题+摘要的向量,用于跨论文语义相似边;为空则不建相似边。
type PaperGraph struct {
	Owner        string
	ID           string
	Title        string
	Year         int
	Venue        string
	Authors      []string
	Keywords     []string
	Affiliations []string
	Embedding    []float64
}

// upsertPaperCypher 幂等写入论文及其内容关系:先 MERGE 论文节点回填属性(含语义向量),
// 再清掉旧的内容边(作者/关键词/机构/会议,不动 CITES/SIMILAR_TO)后按入参重建,保证重解析覆盖。
// 内容节点按归一化键 norm 合并(大小写/空格差异视为同一项),展示名取首次出现的原文。
const upsertPaperCypher = `
MERGE (p:Paper {owner:$owner, id:$id})
SET p.title=$title, p.norm_title=$normTitle, p.year=$year, p.venue=$venue, p.embedding=$embedding, p.updated_at=timestamp()
WITH p
CALL { WITH p OPTIONAL MATCH (p)-[r:AUTHORED_BY|HAS_KEYWORD|FROM_AFFILIATION|PUBLISHED_IN]->() DELETE r }
WITH p
CALL { WITH p UNWIND $authors AS t MERGE (a:Author {owner:$owner, norm:t.norm}) ON CREATE SET a.name=t.name MERGE (p)-[:AUTHORED_BY]->(a) }
WITH p
CALL { WITH p UNWIND $keywords AS t MERGE (k:Keyword {owner:$owner, norm:t.norm}) ON CREATE SET k.name=t.name MERGE (p)-[:HAS_KEYWORD]->(k) }
WITH p
CALL { WITH p UNWIND $affiliations AS t MERGE (af:Affiliation {owner:$owner, norm:t.norm}) ON CREATE SET af.name=t.name MERGE (p)-[:FROM_AFFILIATION]->(af) }
WITH p
CALL { WITH p UNWIND $venues AS t MERGE (v:Venue {owner:$owner, norm:t.norm}) ON CREATE SET v.name=t.name MERGE (p)-[:PUBLISHED_IN]->(v) }
`

// UpsertPaper 幂等写入一篇论文及其作者/关键词/机构/会议节点与关系,随后按语义向量重建相似边。
func UpsertPaper(ctx context.Context, p PaperGraph) error {
	venues := []string{}
	if v := strings.TrimSpace(p.Venue); v != "" {
		venues = []string{v}
	}
	embedding := p.Embedding
	if embedding == nil {
		embedding = []float64{}
	}
	params := map[string]any{
		"owner":        p.Owner,
		"id":           p.ID,
		"title":        strings.TrimSpace(p.Title),
		"normTitle":    normalizeTitle(p.Title),
		"year":         p.Year,
		"venue":        strings.TrimSpace(p.Venue),
		"embedding":    embedding,
		"authors":      cleanTerms(p.Authors),
		"keywords":     cleanTerms(p.Keywords),
		"affiliations": cleanTerms(p.Affiliations),
		"venues":       cleanTerms(venues),
	}
	if _, err := exec(ctx, upsertPaperCypher, params); err != nil {
		return err
	}
	return relinkSimilar(ctx, p.Owner, p.ID)
}

// relinkSimilarDeleteCypher 清掉本论文发出的相似边,供按当前向量重建。
// 不删除其他论文指向本论文的边,避免新论文入库时把旧论文已建立的反向相似关系抹掉。
const relinkSimilarDeleteCypher = `
MATCH (p:Paper {owner:$owner, id:$id})-[s:SIMILAR_TO]->() DELETE s`

// relinkSimilarCypher 用 Neo4j 原生 vector.similarity.cosine 在存储向量上算相似度,
// 超阈值取 Top K 建 SIMILAR_TO。在库内一致计算,避免 Go 侧拿新鲜向量与存储漂移导致算偏。
const relinkSimilarCypher = `
MATCH (p:Paper {owner:$owner, id:$id})
WHERE p.embedding IS NOT NULL AND size(p.embedding) > 0
MATCH (q:Paper {owner:$owner})
WHERE q.id <> p.id AND q.embedding IS NOT NULL AND size(q.embedding) > 0
WITH p, q, vector.similarity.cosine(p.embedding, q.embedding) AS sim
WHERE sim >= $threshold
WITH p, q, sim ORDER BY sim DESC LIMIT $topK
MERGE (p)-[s:SIMILAR_TO]->(q) SET s.score = sim, s.updated_at = timestamp()`

// relinkSimilar 按论文语义向量重建相似边:先清本论文发出的旧边,再与同用户其他论文在库内算 cosine,
// 超过阈值的取 Top K 建 SIMILAR_TO。本论文无向量时只清旧出边(不参与相似召回)。
// 关键词字面难重合的同领域论文(如各篇 attention 论文)靠这条边连上。
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

// upsertCitationsCypher 幂等写入引用关系:清旧 CITES 后,把每条参考文献建成 Reference 桩节点
// 并连 CITES(供共被引);再用归一化串包含匹配命中本用户已有论文标题时建论文到论文的直接引用边。
const upsertCitationsCypher = `
MATCH (p:Paper {owner:$owner, id:$id})
CALL { WITH p OPTIONAL MATCH (p)-[c:CITES]->() DELETE c }
WITH p
CALL { WITH p UNWIND $refs AS ref MERGE (r:Reference {owner:$owner, key:ref.key}) ON CREATE SET r.raw=ref.raw MERGE (p)-[:CITES]->(r) }
WITH p
CALL { WITH p UNWIND $refNorms AS rn MATCH (q:Paper {owner:$owner}) WHERE q.id <> p.id AND q.norm_title <> '' AND rn CONTAINS q.norm_title MERGE (p)-[:CITES]->(q) }
`

// UpsertCitations 幂等写入论文的参考文献引用关系。refs 为参考文献原文串。
func UpsertCitations(ctx context.Context, owner, paperID string, refs []string) error {
	refMaps := make([]map[string]any, 0, len(refs))
	norms := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, raw := range refs {
		raw = strings.TrimSpace(raw)
		norm := normalizeTitle(raw)
		// 太短的条目多为页码/编号噪声,跳过;归一化串做去重键。
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
	if len(refMaps) == 0 {
		// 仍需清掉旧 CITES,传空列表走同一语句即可。
		refMaps = []map[string]any{}
		norms = []string{}
	}
	params := map[string]any{
		"owner":    owner,
		"id":       paperID,
		"refs":     refMaps,
		"refNorms": norms,
	}
	_, err := exec(ctx, upsertCitationsCypher, params)
	return err
}

// DeletePaper 删除一篇论文节点及其所有关系,再清掉因此孤立的内容/参考节点。
func DeletePaper(ctx context.Context, owner, paperID string) error {
	if _, err := exec(ctx, `MATCH (p:Paper {owner:$owner, id:$id}) DETACH DELETE p`,
		map[string]any{"owner": owner, "id": paperID}); err != nil {
		return err
	}
	return CleanupOrphans(ctx, owner)
}

// cleanupOrphansCypher 清掉某用户名下不再被任何论文引用的内容/参考节点。
const cleanupOrphansCypher = `
MATCH (n)
WHERE n.owner=$owner AND (n:Author OR n:Keyword OR n:Affiliation OR n:Venue OR n:Reference) AND NOT (n)--()
DELETE n`

// CleanupOrphans 清理某用户因删除或归一化键变更而孤立的内容节点,best-effort。
func CleanupOrphans(ctx context.Context, owner string) error {
	_, err := exec(ctx, cleanupOrphansCypher, map[string]any{"owner": owner})
	return err
}

// term 是内容节点的展示名与归一化键。norm 为合并键,name 为首次出现的展示文本。
// cleanTerms 去空白与空项,按 norm 去重保持顺序,空 norm 的项丢弃。
func cleanTerms(in []string) []map[string]any {
	seen := map[string]bool{}
	out := make([]map[string]any, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		norm := normalizeTitle(s)
		if s == "" || norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, map[string]any{"name": s, "norm": norm})
	}
	return out
}

// normalizeTitle 把标题/参考文献归一化为小写字母数字加单空格的串,用于跨条目稳定匹配与去重。
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
