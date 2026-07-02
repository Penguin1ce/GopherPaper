package paper

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"GopherPaper/internal/ai"
	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
	"GopherPaper/internal/mq"
	"GopherPaper/internal/parser"
	"GopherPaper/internal/service/metrics"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

// ReparseOptions 控制离线 MinerU 归档重解析。
type ReparseOptions struct {
	PaperID         string
	ContinueOnError bool
	RebuildGraph    bool
}

// ReparseResult 是重解析批处理计数。
type ReparseResult struct {
	Total   int
	Done    int
	Skipped int
	Failed  int
}

// Reparse 重新投递某篇论文的完整解析流水线。
func Reparse(ctx context.Context, ownerID, paperID string) (*model.Paper, error) {
	p, err := owned(ctx, ownerID, paperID)
	if err != nil {
		return nil, err
	}
	if reparseBusy(p.Status) {
		return nil, errs.ErrPaperBusy
	}
	task := taskOf(p)
	if _, err := os.Stat(p.FileURI); err != nil {
		if os.IsNotExist(err) {
			setStatus(ctx, task, constant.PaperFailed, "原始 PDF 文件不存在,无法重新解析")
			return nil, errs.ErrPaperFileMissing
		}
		return nil, fmt.Errorf("service/paper: 检查原始 PDF 失败: %w", err)
	}
	if err := paperdao.DeleteReports(ctx, paperID); err != nil {
		return nil, err
	}
	invalidateReadyCache(ctx, paperID)
	if err := paperdao.UpdateParseProgress(ctx, paperID, 0, 0, 0); err != nil {
		zlog.Warn("重置论文解析进度失败,继续重新解析", "paper_id", paperID, "err", err)
	}
	setStatus(ctx, task, constant.PaperUploaded, "重新解析任务已提交")
	if mqClient != nil {
		if err := publishParse(ctx, p); err != nil {
			zlog.Error("投递重新解析队列失败,改为后台解析", "paper_id", p.ID, "err", err)
			go runPipeline(context.WithoutCancel(ctx), task)
		}
	} else {
		zlog.Warn("MQ 未初始化,改为后台解析", "paper_id", p.ID)
		go runPipeline(context.WithoutCancel(ctx), task)
	}
	p.Status = constant.PaperUploaded
	p.FailReason = "重新解析任务已提交"
	p.ParseProgress = 0
	p.ParsedPages = 0
	p.TotalPages = 0
	return p, nil
}

func reparseBusy(status constant.PaperStatus) bool {
	switch status {
	case constant.PaperParsing, constant.PaperExtracted, constant.PaperIndexed:
		return true
	default:
		return false
	}
}

// EnqueueReparseMinerUArchives 扫描论文 MinerU 归档并投递重解析任务到 MQ。
// 实际重解析由解析 worker 消费 mode=mineru_archive 的消息完成。
func EnqueueReparseMinerUArchives(ctx context.Context, papers []model.Paper, client *mq.Client, queue string, opts ReparseOptions) (ReparseResult, error) {
	var result ReparseResult
	for _, p := range papers {
		if opts.PaperID != "" && p.ID != opts.PaperID {
			continue
		}
		result.Total++
		dir := mineruDir(p.ID)
		ok, err := hasMinerUArchive(dir)
		if err != nil {
			result.Failed++
			zlog.Error("检查 MinerU 归档失败", "paper_id", p.ID, "dir", dir, "err", err)
			if !opts.ContinueOnError {
				return result, err
			}
			continue
		}
		if !ok {
			result.Skipped++
			zlog.Info("跳过重解析投递: 未找到 MinerU 归档", "paper_id", p.ID, "file", p.FileName, "dir", dir)
			continue
		}
		if err := publishReparse(ctx, client, queue, p, opts.RebuildGraph); err != nil {
			result.Failed++
			zlog.Error("投递 MinerU 归档重解析任务失败", "paper_id", p.ID, "queue", queue, "err", err)
			if !opts.ContinueOnError {
				return result, err
			}
			continue
		}
		result.Done++
		zlog.Info("已投递 MinerU 归档重解析任务", "paper_id", p.ID, "file", p.FileName, "queue", queue, "rebuild_graph", opts.RebuildGraph)
	}
	zlog.Info("MinerU 归档重解析任务投递完成", "total", result.Total, "queued", result.Done, "skipped", result.Skipped, "failed", result.Failed)
	return result, nil
}

// ReparseMinerUArchives 遍历论文记录,用 data/papers/mineru/<paperID> 下的 MinerU 归档重建解析结果。
// 缺少归档的论文只打印日志并跳过;不会重新调用 MinerU 在线 API。
func ReparseMinerUArchives(ctx context.Context, papers []model.Paper, opts ReparseOptions) (ReparseResult, error) {
	var result ReparseResult
	for _, p := range papers {
		if opts.PaperID != "" && p.ID != opts.PaperID {
			continue
		}
		result.Total++
		dir := mineruDir(p.ID)
		ok, err := hasMinerUArchive(dir)
		if err != nil {
			result.Failed++
			zlog.Error("检查 MinerU 归档失败", "paper_id", p.ID, "dir", dir, "err", err)
			if !opts.ContinueOnError {
				return result, err
			}
			continue
		}
		if !ok {
			result.Skipped++
			zlog.Info("跳过重解析: 未找到 MinerU 归档", "paper_id", p.ID, "file", p.FileName, "dir", dir)
			continue
		}
		task := taskOf(&p)
		task.RebuildGraph = opts.RebuildGraph
		if err := reparseOneFromMinerU(ctx, task, dir); err != nil {
			result.Failed++
			zlog.Error("MinerU 归档重解析失败", "paper_id", p.ID, "file", p.FileName, "dir", dir, "err", err)
			if !opts.ContinueOnError {
				return result, err
			}
			continue
		}
		result.Done++
	}
	zlog.Info("MinerU 归档重解析完成", "total", result.Total, "done", result.Done, "skipped", result.Skipped, "failed", result.Failed)
	return result, nil
}

func runReparsePipeline(ctx context.Context, task parseTask) {
	dir := mineruDir(task.PaperID)
	uctx := tenant.With(ctx, tenant.Tenant{StudentID: task.OwnerID})
	ok, err := hasMinerUArchive(dir)
	if err != nil {
		zlog.Error("检查 MinerU 归档失败,跳过重解析", "paper_id", task.PaperID, "dir", dir, "err", err)
		fail(uctx, task, "检查 MinerU 归档失败", err, time.Now())
		return
	}
	if !ok {
		zlog.Info("跳过重解析: 未找到 MinerU 归档", "paper_id", task.PaperID, "file", task.FileName, "dir", dir)
		setStatus(uctx, task, constant.PaperFailed, "未找到 MinerU 归档,无法离线重新解析")
		return
	}
	if err := reparseOneFromMinerU(ctx, task, dir); err != nil {
		zlog.Error("MinerU 归档重解析失败", "paper_id", task.PaperID, "file", task.FileName, "dir", dir, "err", err)
	}
}

func publishReparse(ctx context.Context, client *mq.Client, queue string, p model.Paper, rebuildGraph bool) error {
	task := taskOf(&p)
	task.Mode = parseTaskModeMinerUArchive
	task.RebuildGraph = rebuildGraph
	body, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("service/paper: 序列化重解析任务失败: %w", err)
	}
	return client.Publish(ctx, queue, body)
}

func reparseOneFromMinerU(ctx context.Context, task parseTask, dir string) error {
	start := time.Now()
	uctx := tenant.With(ctx, tenant.Tenant{StudentID: task.OwnerID})
	zlog.Info("开始 MinerU 归档重解析", "paper_id", task.PaperID, "owner", task.OwnerID, "dir", dir)
	setStatus(uctx, task, constant.PaperParsing, "从 MinerU 归档重解析")

	doc, err := parser.ParseArtifactDir(dir)
	if err != nil {
		fail(uctx, task, "读取 MinerU 归档失败", err, start)
		return err
	}
	structured, err := ai.Extract(uctx, doc)
	if err != nil {
		fail(uctx, task, "结构化抽取失败", err, start)
		return err
	}
	if !paperPresentForPipeline(uctx, task, "reparse_save_structured") {
		return fmt.Errorf("service/paper: 论文不存在或归属变化")
	}
	if err := saveStructured(uctx, task.PaperID, structured, doc); err != nil {
		fail(uctx, task, "落库元信息失败", err, start)
		return err
	}
	setStatus(uctx, task, constant.PaperExtracted, "MinerU 归档重解析")

	if !paperPresentForPipeline(uctx, task, "reparse_save_figures") {
		return fmt.Errorf("service/paper: 论文不存在或归属变化")
	}
	saveFigures(task, doc)
	if err := ai.DescribeFigures(uctx, doc.Figures); err != nil {
		zlog.Error("图片描述生成失败,降级只用 caption", "paper_id", task.PaperID, "err", err)
	}
	chunks := buildMetaChunks(task, structured)
	chunks = append(chunks, buildChunks(task, doc)...)
	chunks = append(chunks, buildFigureChunks(task, doc)...)
	chunks = append(chunks, buildTableChunks(task, doc)...)
	chunks = append(chunks, buildCodeChunks(task, doc)...)

	if !paperPresentForPipeline(uctx, task, "reparse_upsert_chunks") {
		return fmt.Errorf("service/paper: 论文不存在或归属变化")
	}
	if err := rebuildPaperVectors(uctx, task, chunks, parseTaskModeMinerUArchive); err != nil {
		fail(uctx, task, "重建向量库失败", err, start)
		return err
	}
	setStatus(uctx, task, constant.PaperIndexed, "MinerU 归档重解析")

	if task.RebuildGraph {
		upsertGraph(uctx, task, structured, doc)
	}
	setStatus(uctx, task, constant.PaperReady, "")
	metrics.Record(uctx, metrics.ServiceParse, task.OwnerID, task.PaperID, "", true, time.Since(start), nil)
	zlog.Info("MinerU 归档重解析完成", "paper_id", task.PaperID, "chunks", len(chunks), "duration", time.Since(start))
	return nil
}

func hasMinerUArchive(dir string) (bool, error) {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}
	found := false
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, "content_list.json") || strings.HasSuffix(name, "content_list_v2.json") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found, err
}
