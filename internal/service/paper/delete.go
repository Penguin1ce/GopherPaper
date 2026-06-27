package paper

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/graph"
	"GopherPaper/internal/knowledge"
	chatservice "GopherPaper/internal/service/chat"
	"GopherPaper/internal/zlog"
)

// Delete removes one owned paper and its derived data.
func Delete(ctx context.Context, ownerID, paperID string) error {
	zlog.Info("开始删除论文", "owner", ownerID, "paper_id", paperID)
	p, err := owned(ctx, ownerID, paperID)
	if err != nil {
		zlog.Warn("删除论文归属校验失败", "owner", ownerID, "paper_id", paperID, "err", err)
		return err
	}
	zlog.Info("论文归属校验通过",
		"owner", ownerID,
		"paper_id", paperID,
		"title", p.Title,
		"file_name", p.FileName,
		"file_uri", p.FileURI,
		"status", p.Status,
	)

	zlog.Info("开始删除论文向量 chunks", "owner", ownerID, "paper_id", paperID)
	if err := deletePaperChunksWithVerification(ctx, ownerID, paperID); err != nil {
		zlog.Error("删除论文向量 chunks 失败", "owner", ownerID, "paper_id", paperID, "err", err)
		return err
	}
	zlog.Info("论文向量 chunks 删除完成", "owner", ownerID, "paper_id", paperID)

	zlog.Info("开始删除论文知识图谱节点", "owner", ownerID, "paper_id", paperID)
	if err := graph.DeletePaper(ctx, ownerID, paperID); err != nil {
		zlog.Error("删除论文知识图谱节点失败", "owner", ownerID, "paper_id", paperID, "err", err)
		return err
	}
	zlog.Info("论文知识图谱节点删除完成", "owner", ownerID, "paper_id", paperID)

	// 清掉论文就绪报告的 Redis 缓存(report:ready:<paperID>),best-effort,不阻断删除。
	invalidateReadyCache(ctx, paperID)

	zlog.Info("开始删除论文绑定会话", "owner", ownerID, "paper_id", paperID)
	if err := chatservice.DeleteSessionsForPaper(ctx, ownerID, paperID); err != nil {
		zlog.Error("删除论文绑定会话失败", "owner", ownerID, "paper_id", paperID, "err", err)
		return fmt.Errorf("service/paper: 删除论文会话失败: %w", err)
	}
	zlog.Info("论文绑定会话删除完成", "owner", ownerID, "paper_id", paperID)

	zlog.Info("开始删除论文 PDF 文件", "owner", ownerID, "paper_id", paperID, "path", p.FileURI)
	if err := removeStoredPath(p.FileURI, false); err != nil {
		zlog.Error("删除论文 PDF 文件失败", "owner", ownerID, "paper_id", paperID, "path", p.FileURI, "err", err)
		return err
	}
	zlog.Info("论文 PDF 文件删除完成", "owner", ownerID, "paper_id", paperID, "path", p.FileURI)

	figurePath := figuresDir(paperID)
	zlog.Info("开始删除论文图片目录", "owner", ownerID, "paper_id", paperID, "path", figurePath)
	if err := removeStoredPath(figurePath, true); err != nil {
		zlog.Error("删除论文图片目录失败", "owner", ownerID, "paper_id", paperID, "path", figurePath, "err", err)
		return err
	}
	zlog.Info("论文图片目录删除完成", "owner", ownerID, "paper_id", paperID, "path", figurePath)

	mineruPath := mineruDir(paperID)
	zlog.Info("开始删除论文 MinerU 归档目录", "owner", ownerID, "paper_id", paperID, "path", mineruPath)
	if err := removeStoredPath(mineruPath, true); err != nil {
		zlog.Error("删除论文 MinerU 归档目录失败", "owner", ownerID, "paper_id", paperID, "path", mineruPath, "err", err)
		return err
	}
	zlog.Info("论文 MinerU 归档目录删除完成", "owner", ownerID, "paper_id", paperID, "path", mineruPath)

	zlog.Info("开始删除论文数据库记录", "owner", ownerID, "paper_id", paperID)
	if err := paperdao.Delete(ctx, paperID); err != nil {
		zlog.Error("删除论文数据库记录失败", "owner", ownerID, "paper_id", paperID, "err", err)
		return err
	}
	zlog.Info("论文删除完成", "owner", ownerID, "paper_id", paperID)
	return nil
}

func deletePaperChunksWithVerification(ctx context.Context, ownerID, paperID string) error {
	paperBefore, paperBeforeErr := knowledge.CountPaperChunks(ctx, ownerID, paperID)
	visibleBefore, visibleBeforeErr := knowledge.VectorCount(ctx)
	statsBefore, statsBeforeErr := knowledge.VectorCollectionStatsCount(ctx)
	zlog.Info("paper vector delete metrics before",
		"owner", ownerID,
		"paper_id", paperID,
		"paper_chunks_before", countOrNil(paperBefore, paperBeforeErr),
		"paper_chunks_before_err", errText(paperBeforeErr),
		"visible_vectors_before", countOrNil(visibleBefore, visibleBeforeErr),
		"visible_vectors_before_err", errText(visibleBeforeErr),
		"collection_stats_before", countOrNil(statsBefore, statsBeforeErr),
		"collection_stats_before_err", errText(statsBeforeErr),
	)
	if paperBeforeErr == nil && paperBefore == 0 {
		zlog.Warn("paper vector delete matched zero chunks before deletion",
			"owner", ownerID,
			"paper_id", paperID,
		)
	}

	if err := knowledge.DeletePaperChunks(ctx, ownerID, paperID); err != nil {
		return err
	}

	paperAfter, paperAfterErr := waitForPaperChunksDeleted(ctx, ownerID, paperID)
	visibleAfter, visibleAfterErr := knowledge.VectorCount(ctx)
	statsAfter, statsAfterErr := knowledge.VectorCollectionStatsCount(ctx)
	zlog.Info("paper vector delete metrics after",
		"owner", ownerID,
		"paper_id", paperID,
		"paper_chunks_before", countOrNil(paperBefore, paperBeforeErr),
		"paper_chunks_after", countOrNil(paperAfter, paperAfterErr),
		"paper_chunks_after_err", errText(paperAfterErr),
		"visible_vectors_before", countOrNil(visibleBefore, visibleBeforeErr),
		"visible_vectors_after", countOrNil(visibleAfter, visibleAfterErr),
		"visible_vectors_after_err", errText(visibleAfterErr),
		"collection_stats_before", countOrNil(statsBefore, statsBeforeErr),
		"collection_stats_after", countOrNil(statsAfter, statsAfterErr),
		"collection_stats_after_err", errText(statsAfterErr),
	)
	if paperAfterErr != nil {
		zlog.Warn("paper vector delete verification failed",
			"owner", ownerID,
			"paper_id", paperID,
			"err", paperAfterErr,
		)
		return nil
	}
	if paperAfter > 0 {
		return fmt.Errorf("service/paper: paper vector chunks remain after delete: before=%d after=%d", paperBefore, paperAfter)
	}
	return nil
}

func waitForPaperChunksDeleted(ctx context.Context, ownerID, paperID string) (int64, error) {
	delays := []time.Duration{0, 200 * time.Millisecond, 500 * time.Millisecond, time.Second}
	var last int64
	for _, delay := range delays {
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return last, ctx.Err()
			}
		}
		n, err := knowledge.CountPaperChunks(ctx, ownerID, paperID)
		if err != nil || n == 0 {
			return n, err
		}
		last = n
	}
	return last, nil
}

func countOrNil(n int64, err error) any {
	if err != nil {
		return nil
	}
	return n
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func removeStoredPath(path string, dir bool) error {
	path = strings.TrimSpace(path)
	if path == "" {
		zlog.Debug("删除存储路径为空,跳过", "dir", dir)
		return nil
	}
	absBase, err := filepath.Abs(storageDir)
	if err != nil {
		zlog.Error("解析存储目录失败", "storage_dir", storageDir, "err", err)
		return fmt.Errorf("service/paper: 解析存储目录失败: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		zlog.Error("解析删除路径失败", "path", path, "dir", dir, "err", err)
		return fmt.Errorf("service/paper: 解析删除路径失败: %w", err)
	}
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		zlog.Error("校验删除路径失败", "storage_dir", absBase, "path", absPath, "dir", dir, "err", err)
		return fmt.Errorf("service/paper: 校验删除路径失败: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		zlog.Error("拒绝删除存储目录外路径", "storage_dir", absBase, "path", absPath, "rel", rel, "dir", dir)
		return fmt.Errorf("service/paper: 拒绝删除存储目录外路径: %s", path)
	}
	if dir {
		if _, err := os.Stat(absPath); errors.Is(err, os.ErrNotExist) {
			zlog.Warn("删除存储目录时目录不存在,视为成功", "path", absPath, "rel", rel)
			return nil
		} else if err != nil {
			zlog.Error("检查存储目录失败", "path", absPath, "rel", rel, "err", err)
			return fmt.Errorf("service/paper: 检查图片目录失败: %w", err)
		}
		zlog.Info("删除存储目录", "path", absPath, "rel", rel)
		if err := os.RemoveAll(absPath); err != nil {
			zlog.Error("删除存储目录失败", "path", absPath, "rel", rel, "err", err)
			return fmt.Errorf("service/paper: 删除图片目录失败: %w", err)
		}
		zlog.Info("存储目录删除完成", "path", absPath, "rel", rel)
		return nil
	}
	zlog.Info("删除存储文件", "path", absPath, "rel", rel)
	if err := os.Remove(absPath); errors.Is(err, os.ErrNotExist) {
		zlog.Warn("删除存储文件时文件不存在,视为成功", "path", absPath, "rel", rel)
		return nil
	} else if err != nil {
		zlog.Error("删除存储文件失败", "path", absPath, "rel", rel, "err", err)
		return fmt.Errorf("service/paper: 删除 PDF 文件失败: %w", err)
	}
	zlog.Info("存储文件删除完成", "path", absPath, "rel", rel)
	return nil
}
