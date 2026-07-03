package admin

import (
	"context"
	"strings"

	"GopherPaper/internal/dto"
)

// 单次批量操作的上限,防止一次删除过多。
const batchLimit = 200

// BatchDeletePapers 批量删除论文,逐个复用 DeletePaper 的级联清理逻辑,
// 汇总成功/失败计数,单个失败不影响其余。
func BatchDeletePapers(ctx context.Context, ids []string) *dto.AdminBatchResult {
	res := &dto.AdminBatchResult{Requested: len(ids)}
	seen := make(map[string]struct{}, len(ids))
	count := 0
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		count++
		if count > batchLimit {
			res.Failed++
			res.FailedIDs = append(res.FailedIDs, id)
			continue
		}
		if err := DeletePaper(ctx, id); err != nil {
			res.Failed++
			res.FailedIDs = append(res.FailedIDs, id)
			continue
		}
		res.Succeeded++
	}
	return res
}
