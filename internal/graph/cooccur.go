package graph

import "context"

// KeywordNode 是关键词共现网的一个节点,Community 为聚类社区号。
type KeywordNode struct {
	ID        string `json:"id"`        // = norm,稳定键
	Label     string `json:"label"`     // = name,展示名
	Count     int    `json:"count"`     // 出现的论文数(频次)
	Community int    `json:"community"` // 社区号
}

// KeywordEdge 是两关键词的共现边,Weight 为共现论文数。
type KeywordEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Weight int    `json:"weight"`
}

// KeywordNetwork 是关键词共现网络,空网络序列化为 [] 而非 null。
type KeywordNetwork struct {
	Nodes []KeywordNode `json:"nodes"`
	Edges []KeywordEdge `json:"edges"`
}

// KeywordCoNetwork 计算某用户 Top-N 高频关键词的共现网络并做社区聚类。
// 共现在读时用 Cypher 计算,不落持久化边;聚类走 Go 侧 detectCommunities。
func KeywordCoNetwork(ctx context.Context, owner string, top, minWeight int) (KeywordNetwork, error) {
	if top <= 0 {
		top = 60
	}
	if minWeight <= 0 {
		minWeight = 1
	}
	net := KeywordNetwork{Nodes: []KeywordNode{}, Edges: []KeywordEdge{}}

	const topCypher = `
MATCH (:Paper {owner:$owner})-[:HAS_KEYWORD]->(k:Keyword {owner:$owner})
WITH k, count(*) AS freq ORDER BY freq DESC LIMIT $top
RETURN k.norm AS norm, k.name AS name, freq`
	res, err := exec(ctx, topCypher, map[string]any{"owner": owner, "top": top})
	if err != nil {
		return KeywordNetwork{}, err
	}
	norms := make([]string, 0, len(res.Records))
	countByNorm := map[string]int{}
	nameByNorm := map[string]string{}
	for _, r := range res.Records {
		norm := asStr(r, "norm")
		if norm == "" {
			continue
		}
		norms = append(norms, norm)
		countByNorm[norm] = asInt(r, "freq")
		nameByNorm[norm] = asStr(r, "name")
	}
	if len(norms) == 0 {
		return net, nil
	}

	const edgeCypher = `
MATCH (k1:Keyword {owner:$owner})<-[:HAS_KEYWORD]-(p:Paper {owner:$owner})-[:HAS_KEYWORD]->(k2:Keyword {owner:$owner})
WHERE k1.norm < k2.norm AND k1.norm IN $norms AND k2.norm IN $norms
WITH k1.norm AS a, k2.norm AS b, count(DISTINCT p) AS w
WHERE w >= $minWeight
RETURN a, b, w`
	eres, err := exec(ctx, edgeCypher, map[string]any{"owner": owner, "norms": norms, "minWeight": minWeight})
	if err != nil {
		return KeywordNetwork{}, err
	}
	wedges := make([]weightedEdge, 0, len(eres.Records))
	for _, r := range eres.Records {
		a := asStr(r, "a")
		b := asStr(r, "b")
		w := asInt(r, "w")
		if a == "" || b == "" {
			continue
		}
		wedges = append(wedges, weightedEdge{A: a, B: b, W: w})
		net.Edges = append(net.Edges, KeywordEdge{Source: a, Target: b, Weight: w})
	}

	community := detectCommunities(norms, wedges)
	for _, norm := range norms {
		net.Nodes = append(net.Nodes, KeywordNode{
			ID:        norm,
			Label:     nameByNorm[norm],
			Count:     countByNorm[norm],
			Community: community[norm],
		})
	}
	return net, nil
}
