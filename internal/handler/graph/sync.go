package graph

import (
	"context"
	"sync"

	paperdao "GopherPaper/internal/dao/paper"
	graphstore "GopherPaper/internal/graph"
	"GopherPaper/internal/zlog"
)

// papersNeedingSync 返回图谱中缺失或已过期(MySQL 元信息比图谱节点新)的 paperID。
// graphState: 图谱现有 Paper 节点 id→updated_at(毫秒);stamps: MySQL 侧每篇 meta 更新时间。
func papersNeedingSync(graphState map[string]int64, stamps []paperdao.MetaStamp) []string {
	ids := make([]string, 0)
	for _, s := range stamps {
		graphMillis, ok := graphState[s.PaperID]
		if !ok || s.UpdatedAt.UnixMilli() > graphMillis {
			ids = append(ids, s.PaperID)
		}
	}
	return ids
}

// ownerSyncLocks 给每个 owner 一把锁,避免多请求并发重复同步(同步本身幂等,加锁只为省开销)。
var ownerSyncLocks sync.Map // owner string -> *sync.Mutex

func lockOwnerSync(owner string) func() {
	m, _ := ownerSyncLocks.LoadOrStore(owner, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// syncDiff 取图谱与 MySQL 现状,把缺失/过期的论文逐篇重写图谱。filter 非空时只看该 paperID。
func syncDiff(ctx context.Context, owner, onlyPaperID string) {
	unlock := lockOwnerSync(owner)
	defer unlock()

	graphState, err := graphstore.PaperSyncState(ctx, owner)
	if err != nil {
		zlog.Error("查询图谱同步状态失败", "owner", owner, "err", err)
		return
	}
	stamps, err := paperdao.ListMetaTimestamps(ctx, owner)
	if err != nil {
		zlog.Error("查询元信息时间戳失败", "owner", owner, "err", err)
		return
	}
	if onlyPaperID != "" {
		filtered := stamps[:0:0]
		for _, s := range stamps {
			if s.PaperID == onlyPaperID {
				filtered = append(filtered, s)
				break
			}
		}
		stamps = filtered
	}
	for _, id := range papersNeedingSync(graphState, stamps) {
		if err := repairPaperGraphFromMeta(ctx, owner, id); err != nil {
			zlog.Error("按需同步论文图谱失败", "owner", owner, "paper_id", id, "err", err)
		}
	}
}

// syncOwnerGraphIfNeeded 只把缺失/过期的论文重新写入图谱,替代读时全量重建。
func syncOwnerGraphIfNeeded(ctx context.Context, owner string) { syncDiff(ctx, owner, "") }

// syncPaperGraphIfNeeded 只在该论文缺失或过期时才重建其图谱。
func syncPaperGraphIfNeeded(ctx context.Context, owner, paperID string) {
	syncDiff(ctx, owner, paperID)
}
