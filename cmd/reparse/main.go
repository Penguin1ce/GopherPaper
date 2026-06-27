// Command reparse queues paper reparse jobs from archived MinerU artifacts.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"flag"

	"GopherPaper/internal/config"
	"GopherPaper/internal/dao"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
	"GopherPaper/internal/mq"
	paperservice "GopherPaper/internal/service/paper"
	"GopherPaper/internal/zlog"
)

func main() {
	cfgPath := flag.String("c", "config/config.toml", "配置文件路径")
	paperID := flag.String("paper-id", "", "只重解析指定论文 ID,留空则扫描全部论文")
	skipGraph := flag.Bool("skip-graph", false, "跳过 Neo4j 知识图谱重建")
	stopOnError := flag.Bool("stop-on-error", false, "单篇失败后立即停止")
	flag.Parse()

	if err := run(*cfgPath, *paperID, *skipGraph, *stopOnError); err != nil {
		zlog.Error("MinerU 归档重解析失败", "err", err)
		os.Exit(1)
	}
}

func run(cfgPath, paperID string, skipGraph, stopOnError bool) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if err := zlog.Init(cfg.Log.Level, cfg.Log.File, cfg.Log.MaxSizeMB, cfg.Log.MaxBackups); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := dao.InitMySQL(cfg.MySQL); err != nil {
		return err
	}
	if err := model.AutoMigrate(dao.DB); err != nil {
		return err
	}
	rebuildGraph := false
	if skipGraph {
		zlog.Info("按参数跳过 Neo4j 知识图谱重建")
	} else if cfg.Neo4j.URI == "" {
		zlog.Warn("Neo4j 未配置,跳过知识图谱重建")
	} else {
		rebuildGraph = true
	}
	mqClient, err := mq.New(cfg.MQ)
	if err != nil {
		return err
	}
	defer mqClient.Close()

	papers, err := targetPapers(ctx, paperID)
	if err != nil {
		return err
	}
	result, err := paperservice.EnqueueReparseMinerUArchives(ctx, papers, mqClient, cfg.MQ.ParseQueue, paperservice.ReparseOptions{
		PaperID:         paperID,
		ContinueOnError: !stopOnError,
		RebuildGraph:    rebuildGraph,
	})
	if err != nil {
		return err
	}
	if result.Failed > 0 {
		return fmt.Errorf("MinerU 归档重解析任务投递存在失败: total=%d queued=%d skipped=%d failed=%d", result.Total, result.Done, result.Skipped, result.Failed)
	}
	return nil
}

func targetPapers(ctx context.Context, paperID string) ([]model.Paper, error) {
	if paperID != "" {
		p, err := paperdao.Get(ctx, paperID)
		if err != nil {
			return nil, err
		}
		return []model.Paper{*p}, nil
	}
	return paperdao.ListAll(ctx)
}
