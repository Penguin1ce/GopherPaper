package paper

import (
	"path/filepath"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/parser"
	"GopherPaper/internal/zlog"
)

// mineruDir 返回某篇论文 MinerU 产物的归档目录 data/papers/mineru/<paperID>。
// 内含 content_list.json 等原始产物,供后续重建索引/重解析离线读取。
func mineruDir(paperID string) string {
	return filepath.Join(storageDir, "mineru", paperID)
}

// saveArtifact 把 MinerU 原始产物解压归档到 mineruDir。
// best-effort:失败只记日志不阻断本次入库(归档是为重建铺垫,缺失不影响当前结果)。
func saveArtifact(task parseTask, doc *core.ParsedDoc) {
	if len(doc.Artifact) == 0 {
		return
	}
	dir := mineruDir(task.PaperID)
	if err := parser.SaveArtifact(dir, doc.Artifact); err != nil {
		zlog.Error("MinerU 产物归档失败,跳过", "paper_id", task.PaperID, "err", err)
		return
	}
	zlog.Info("MinerU 产物已归档", "paper_id", task.PaperID, "dir", dir)
}
