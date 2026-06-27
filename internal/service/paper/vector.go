package paper

import (
	"context"
	"fmt"
	"time"

	"GopherPaper/internal/knowledge"
	"GopherPaper/internal/zlog"
)

func rebuildPaperVectors(ctx context.Context, task parseTask, chunks []knowledge.Chunk, mode string) error {
	fields := []any{
		"paper_id", task.PaperID,
		"owner", task.OwnerID,
		"mode", mode,
		"new_chunks", len(chunks),
	}
	oldCount, err := knowledge.CountPaperChunks(ctx, task.OwnerID, task.PaperID)
	if err != nil {
		zlog.Warn("统计论文旧向量 chunks 失败,继续重建", append(fields, "err", err)...)
	} else {
		fields = append(fields, "old_chunks", oldCount)
		zlog.Info("开始重建论文向量 chunks", fields...)
	}

	start := time.Now()
	if err := knowledge.DeletePaperChunks(ctx, task.OwnerID, task.PaperID); err != nil {
		return fmt.Errorf("清理旧 chunks 失败: %w", err)
	}
	zlog.Info("论文旧向量 chunks 清理完成", fields...)

	ids, err := knowledge.UpsertChunks(ctx, chunks)
	if err != nil {
		return fmt.Errorf("写入向量库失败: %w", err)
	}

	currentCount, countErr := waitForPaperVectorCount(ctx, task.OwnerID, task.PaperID, int64(len(ids)))
	doneFields := append(fields,
		"written_chunks", len(ids),
		"duration", time.Since(start),
	)
	if countErr != nil {
		zlog.Warn("统计论文新向量 chunks 失败", append(doneFields, "err", countErr)...)
		return nil
	}
	if currentCount != int64(len(ids)) {
		zlog.Warn("论文向量 chunks 重建完成,可见数量暂未收敛", append(doneFields, "current_chunks", currentCount)...)
		return nil
	}
	zlog.Info("论文向量 chunks 重建完成", append(doneFields, "current_chunks", currentCount)...)
	return nil
}

func waitForPaperVectorCount(ctx context.Context, ownerID, paperID string, want int64) (int64, error) {
	var last int64
	for attempt := 0; attempt < 8; attempt++ {
		n, err := knowledge.CountPaperChunks(ctx, ownerID, paperID)
		if err != nil {
			return 0, err
		}
		last = n
		if n == want {
			return n, nil
		}
		if attempt == 7 {
			break
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 200 * time.Millisecond):
		}
	}
	return last, nil
}
