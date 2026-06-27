package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	mysqlcfg "github.com/go-sql-driver/mysql"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/redis/go-redis/v9"

	"GopherPaper/internal/config"
)

type opts struct {
	configPath string
	yes        bool
	mysql      bool
	redis      bool
	milvus     bool
	neo4j      bool
	papers     bool
	mysqlMode  string
	paperDir   string
}

func main() {
	var o opts
	flag.StringVar(&o.configPath, "c", "config/config.toml", "config path")
	flag.BoolVar(&o.yes, "yes", false, "run destructive reset")
	flag.BoolVar(&o.mysql, "mysql", true, "reset MySQL")
	flag.BoolVar(&o.redis, "redis", true, "reset Redis")
	flag.BoolVar(&o.milvus, "milvus", true, "reset Milvus")
	flag.BoolVar(&o.neo4j, "neo4j", true, "reset Neo4j knowledge graph")
	flag.BoolVar(&o.papers, "papers", true, "reset local uploaded paper files")
	flag.StringVar(&o.mysqlMode, "mysql-mode", "drop", "drop or truncate")
	flag.StringVar(&o.paperDir, "paper-dir", "data/papers", "local uploaded paper directory")
	flag.Parse()

	if err := run(o); err != nil {
		fmt.Fprintf(os.Stderr, "reset failed: %v\n", err)
		os.Exit(1)
	}
}

func run(o opts) error {
	if o.mysqlMode != "drop" && o.mysqlMode != "truncate" {
		return fmt.Errorf("mysql-mode must be drop or truncate")
	}
	cfg, err := config.Load(o.configPath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	printPlan(cfg, o)
	if !o.yes {
		fmt.Println("dry run only, add --yes to reset data")
		return nil
	}

	if o.mysql {
		if err := resetMySQL(ctx, cfg.MySQL.DSN, o.mysqlMode); err != nil {
			return err
		}
	}
	if o.redis {
		if err := resetRedis(ctx, cfg.Redis); err != nil {
			return err
		}
	}
	if o.milvus {
		if err := resetMilvus(ctx, cfg.Milvus); err != nil {
			return err
		}
	}
	if o.neo4j {
		if err := resetNeo4j(ctx, cfg.Neo4j); err != nil {
			return err
		}
	}
	if o.papers {
		if err := resetPaperFiles(o.paperDir); err != nil {
			return err
		}
	}
	fmt.Println("reset complete")
	return nil
}

func printPlan(cfg *config.Config, o opts) {
	fmt.Println("reset targets")
	fmt.Printf("config: %s\n", o.configPath)
	if o.mysql {
		fmt.Printf("mysql: %s, mode=%s\n", describeDSN(cfg.MySQL.DSN), o.mysqlMode)
	}
	if o.redis {
		fmt.Printf("redis: %s db=%d\n", cfg.Redis.Addr, cfg.Redis.DB)
	}
	if o.milvus {
		fmt.Printf("milvus: %s collection=%s\n", cfg.Milvus.Address, cfg.Milvus.Collection)
	}
	if o.neo4j {
		fmt.Printf("neo4j: %s db=%s\n", cfg.Neo4j.URI, neo4jDBName(cfg.Neo4j.Database))
	}
	if o.papers {
		fmt.Printf("papers: %s\n", o.paperDir)
	}
}

func resetMySQL(ctx context.Context, dsn, mode string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("mysql open: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("mysql ping: %w", err)
	}

	tables, err := listTables(ctx, db)
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		fmt.Println("mysql: no tables")
		return nil
	}

	if _, err := db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return fmt.Errorf("mysql disable foreign keys: %w", err)
	}
	defer db.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS = 1")

	for _, table := range tables {
		stmt := "DROP TABLE IF EXISTS " + quoteMySQLIdent(table)
		if mode == "truncate" {
			stmt = "TRUNCATE TABLE " + quoteMySQLIdent(table)
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("mysql reset table %s: %w", table, err)
		}
		fmt.Printf("mysql: %s %s\n", mode, table)
	}
	return nil
}

func listTables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SHOW FULL TABLES")
	if err != nil {
		return nil, fmt.Errorf("mysql list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		var typ string
		if err := rows.Scan(&name, &typ); err != nil {
			return nil, fmt.Errorf("mysql scan table: %w", err)
		}
		if strings.EqualFold(typ, "BASE TABLE") {
			tables = append(tables, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql table rows: %w", err)
	}
	sort.Strings(tables)
	return tables, nil
}

func resetRedis(ctx context.Context, cfg config.RedisConfig) error {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	if err := rdb.FlushDB(ctx).Err(); err != nil {
		return fmt.Errorf("redis flush db: %w", err)
	}
	fmt.Printf("redis: flush db %d\n", cfg.DB)
	return nil
}

func resetMilvus(ctx context.Context, cfg config.MilvusConfig) error {
	collection := strings.TrimSpace(cfg.Collection)
	if collection == "" {
		return fmt.Errorf("milvus collection is empty")
	}
	cli, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address:  cfg.Address,
		Username: cfg.Username,
		Password: cfg.Password,
	})
	if err != nil {
		return fmt.Errorf("milvus connect: %w", err)
	}
	defer cli.Close(ctx)

	has, err := cli.HasCollection(ctx, milvusclient.NewHasCollectionOption(collection))
	if err != nil {
		return fmt.Errorf("milvus has collection %s: %w", collection, err)
	}
	if !has {
		fmt.Printf("milvus: collection %s not found\n", collection)
		return nil
	}
	if err := cli.DropCollection(ctx, milvusclient.NewDropCollectionOption(collection)); err != nil {
		return fmt.Errorf("milvus drop collection %s: %w", collection, err)
	}
	fmt.Printf("milvus: drop collection %s\n", collection)
	return nil
}

// resetNeo4j 清空知识图谱:只删本项目用到的标签节点(连带关系),不动其它数据库里的图。
func resetNeo4j(ctx context.Context, cfg config.Neo4jConfig) error {
	driver, err := neo4j.NewDriverWithContext(cfg.URI, neo4j.BasicAuth(cfg.Username, cfg.Password, ""))
	if err != nil {
		return fmt.Errorf("neo4j connect: %w", err)
	}
	defer driver.Close(ctx)
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return fmt.Errorf("neo4j ping: %w", err)
	}

	opts := []neo4j.ExecuteQueryConfigurationOption{}
	if db := strings.TrimSpace(cfg.Database); db != "" {
		opts = append(opts, neo4j.ExecuteQueryWithDatabase(db))
	}
	const cypher = `MATCH (n) WHERE n:Paper OR n:Author OR n:Keyword OR n:Affiliation OR n:Venue OR n:ResearchQuestion OR n:Method OR n:Experiment OR n:Result OR n:Innovation OR n:Limitation OR n:FutureWork OR n:Reference DETACH DELETE n`
	res, err := neo4j.ExecuteQuery(ctx, driver, cypher, nil, neo4j.EagerResultTransformer, opts...)
	if err != nil {
		return fmt.Errorf("neo4j reset: %w", err)
	}
	fmt.Printf("neo4j: delete %d nodes\n", res.Summary.Counters().NodesDeleted())
	return nil
}

func resetPaperFiles(dir string) error {
	target, err := projectScopedDir(dir)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(target)
	if os.IsNotExist(err) {
		fmt.Printf("papers: directory %s not found\n", dir)
		return nil
	}
	if err != nil {
		return fmt.Errorf("papers read dir %s: %w", dir, err)
	}
	for _, entry := range entries {
		path := filepath.Join(target, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("papers remove %s: %w", path, err)
		}
	}
	fmt.Printf("papers: remove %d entries from %s\n", len(entries), dir)
	return nil
}

func projectScopedDir(dir string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(dir))
	if clean == "" || clean == "." || clean == string(filepath.Separator) {
		return "", fmt.Errorf("papers directory is unsafe: %q", dir)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get cwd: %w", err)
	}
	target, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("resolve papers directory: %w", err)
	}
	rel, err := filepath.Rel(cwd, target)
	if err != nil {
		return "", fmt.Errorf("check papers directory: %w", err)
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("papers directory must be inside project: %s", dir)
	}
	if rel != filepath.Join("data", "papers") && rel != filepath.Join("data", "paper") {
		return "", fmt.Errorf("papers directory must be data/papers or data/paper, got %s", rel)
	}
	return target, nil
}

func describeDSN(dsn string) string {
	cfg, err := mysqlcfg.ParseDSN(dsn)
	if err != nil {
		return "configured mysql dsn"
	}
	return fmt.Sprintf("%s@%s(%s)/%s", cfg.User, cfg.Net, cfg.Addr, cfg.DBName)
}

// neo4jDBName 给计划打印兜底库名,留空时走 Neo4j 默认库 neo4j。
func neo4jDBName(db string) string {
	if d := strings.TrimSpace(db); d != "" {
		return d
	}
	return "neo4j (default)"
}

func quoteMySQLIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}
