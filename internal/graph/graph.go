// Package graph 是知识图谱存储层,独占 Neo4j 驱动,把论文与作者/关键词/机构/参考文献
// 建成关系图,支撑论文间关系发现与按年份的研究趋势。定位对标 knowledge:包级 Init
// 拉起驱动并建约束,其他包直接调本包的写入/查询/删除函数。图谱按用户隔离,所有节点
// 带 owner 属性,查询一律按 owner 过滤。
package graph

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"GopherPaper/internal/config"
)

// driver 是全局 Neo4j 驱动,由 Init 初始化。database 为目标库,空走默认库。
var (
	driver   neo4j.DriverWithContext
	database string
)

// schemaStatements 是建表期一次性执行的约束与索引,均幂等。
// 内容节点唯一约束按 (owner, norm) 归一化键去重(大小写/空格差异视为同一项),
// 故先弃用旧的 (owner, name) 约束;年份/owner 索引服务趋势查询。
var schemaStatements = []string{
	// 旧版按原文 name 建的唯一约束已弃用,改归一化键 norm,先丢旧约束再建新的。
	"DROP CONSTRAINT author_owner_name IF EXISTS",
	"DROP CONSTRAINT keyword_owner_name IF EXISTS",
	"DROP CONSTRAINT affiliation_owner_name IF EXISTS",
	"DROP CONSTRAINT venue_owner_name IF EXISTS",
	"CREATE CONSTRAINT paper_owner_id IF NOT EXISTS FOR (p:Paper) REQUIRE (p.owner, p.id) IS UNIQUE",
	"CREATE CONSTRAINT author_owner_norm IF NOT EXISTS FOR (a:Author) REQUIRE (a.owner, a.norm) IS UNIQUE",
	"CREATE CONSTRAINT keyword_owner_norm IF NOT EXISTS FOR (k:Keyword) REQUIRE (k.owner, k.norm) IS UNIQUE",
	"CREATE CONSTRAINT affiliation_owner_norm IF NOT EXISTS FOR (a:Affiliation) REQUIRE (a.owner, a.norm) IS UNIQUE",
	"CREATE CONSTRAINT venue_owner_norm IF NOT EXISTS FOR (v:Venue) REQUIRE (v.owner, v.norm) IS UNIQUE",
	"CREATE CONSTRAINT research_question_owner_norm IF NOT EXISTS FOR (q:ResearchQuestion) REQUIRE (q.owner, q.norm) IS UNIQUE",
	"CREATE CONSTRAINT method_owner_norm IF NOT EXISTS FOR (m:Method) REQUIRE (m.owner, m.norm) IS UNIQUE",
	"CREATE CONSTRAINT experiment_owner_norm IF NOT EXISTS FOR (e:Experiment) REQUIRE (e.owner, e.norm) IS UNIQUE",
	"CREATE CONSTRAINT result_owner_norm IF NOT EXISTS FOR (r:Result) REQUIRE (r.owner, r.norm) IS UNIQUE",
	"CREATE CONSTRAINT innovation_owner_norm IF NOT EXISTS FOR (i:Innovation) REQUIRE (i.owner, i.norm) IS UNIQUE",
	"CREATE CONSTRAINT limitation_owner_norm IF NOT EXISTS FOR (l:Limitation) REQUIRE (l.owner, l.norm) IS UNIQUE",
	"CREATE CONSTRAINT future_work_owner_norm IF NOT EXISTS FOR (f:FutureWork) REQUIRE (f.owner, f.norm) IS UNIQUE",
	"CREATE CONSTRAINT reference_owner_key IF NOT EXISTS FOR (r:Reference) REQUIRE (r.owner, r.key) IS UNIQUE",
	"CREATE INDEX paper_owner IF NOT EXISTS FOR (p:Paper) ON (p.owner)",
	"CREATE INDEX paper_year IF NOT EXISTS FOR (p:Paper) ON (p.year)",
}

// Init 建驱动并校验连通,随后建约束与索引。须在配置加载后调用一次。
func Init(cfg config.Neo4jConfig) error {
	d, err := neo4j.NewDriverWithContext(cfg.URI, neo4j.BasicAuth(cfg.Username, cfg.Password, ""))
	if err != nil {
		return fmt.Errorf("graph: 建 Neo4j 驱动失败: %w", err)
	}
	ctx := context.Background()
	if err := d.VerifyConnectivity(ctx); err != nil {
		return fmt.Errorf("graph: Neo4j 连通校验失败: %w", err)
	}
	driver = d
	database = cfg.Database
	for _, stmt := range schemaStatements {
		if _, err := exec(ctx, stmt, nil); err != nil {
			return fmt.Errorf("graph: 建约束失败 %q: %w", stmt, err)
		}
	}
	return nil
}

// Close 释放驱动连接,进程退出前调用。
func Close() {
	if driver != nil {
		_ = driver.Close(context.Background())
	}
}

// exec 跑一条 Cypher,自动按 database 路由,返回急加载结果。写读统一走此入口。
func exec(ctx context.Context, cypher string, params map[string]any) (*neo4j.EagerResult, error) {
	opts := []neo4j.ExecuteQueryConfigurationOption{}
	if database != "" {
		opts = append(opts, neo4j.ExecuteQueryWithDatabase(database))
	}
	return neo4j.ExecuteQuery(ctx, driver, cypher, params, neo4j.EagerResultTransformer, opts...)
}

// asInt 从记录取整数值,缺失或类型不符返回 0。Neo4j 整数回传为 int64。
func asInt(rec *neo4j.Record, key string) int {
	v, ok := rec.Get(key)
	if !ok || v == nil {
		return 0
	}
	if n, ok := v.(int64); ok {
		return int(n)
	}
	return 0
}

// asStr 从记录取字符串值,缺失返回空串。
func asStr(rec *neo4j.Record, key string) string {
	v, ok := rec.Get(key)
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// asStrSlice 从记录取字符串列表,Neo4j 列表回传为 []any。
func asStrSlice(rec *neo4j.Record, key string) []string {
	v, ok := rec.Get(key)
	if !ok || v == nil {
		return nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, e := range raw {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
