package paper

import (
	"os"
	"path/filepath"
	"strings"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/knowledge"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// figuresDir 返回某篇论文图片的落盘目录 data/papers/figures/<paperID>。
func figuresDir(paperID string) string {
	return filepath.Join(storageDir, "figures", paperID)
}

// FigurePath 拼出某篇论文某张图的本地路径,取图接口据此读文件。name 已在 handler 做过防穿越。
func FigurePath(paperID, name string) string {
	return filepath.Join(figuresDir(paperID), name)
}

// saveFigures 把解析出的图片字节落盘到论文图片目录,并回填每个 Figure 的 ImgURI。
// best-effort:单图失败只记日志跳过,不阻断整篇入库(图片是增强,缺失不致命)。
func saveFigures(task parseTask, doc *core.ParsedDoc) {
	dir := figuresDir(task.PaperID)
	made := false
	for i := range doc.Figures {
		fig := &doc.Figures[i]
		if len(fig.ImgData) == 0 || fig.ImgPath == "" {
			continue
		}
		if !made {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				zlog.Error("创建图片目录失败,跳过本篇图片", "paper_id", task.PaperID, "err", err)
				return
			}
			made = true
		}
		name := filepath.Base(fig.ImgPath)
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, fig.ImgData, 0o644); err != nil {
			zlog.Error("图片落盘失败,跳过", "paper_id", task.PaperID, "name", name, "err", err)
			continue
		}
		fig.ImgURI = path
	}
}

// buildFigureChunks 把图片建成图块入库:caption + vlm 描述一起进向量供召回,
// metadata 标 block_type=image 并带 img_uri,命中后供带图问答与前端回显。
// caption 与描述都空的图无从召回,跳过。
func buildFigureChunks(task parseTask, doc *core.ParsedDoc) []knowledge.Chunk {
	chunks := make([]knowledge.Chunk, 0, len(doc.Figures))
	for _, fig := range doc.Figures {
		content := figureContent(fig)
		if content == "" {
			continue
		}
		chunks = append(chunks, knowledge.Chunk{
			Content:    content,
			Scope:      constant.KnowledgeScopePrivate,
			OwnerID:    task.OwnerID,
			DocID:      task.PaperID,
			SourceFile: task.FileName,
			PageNo:     int64(fig.PageNo),
			ChunkIndex: int64(len(chunks)),
			Metadata: map[string]any{
				constant.MilvusFieldBlockType: constant.BlockTypeImage,
				constant.MilvusFieldImgURI:    fig.ImgURI,
			},
		})
	}
	return chunks
}

// figureContent 把 caption 与 vlm 描述拼成图块向量化文本,两者去空合并。
func figureContent(fig core.Figure) string {
	parts := make([]string, 0, 2)
	if c := strings.TrimSpace(fig.Caption); c != "" {
		parts = append(parts, c)
	}
	if d := strings.TrimSpace(fig.Desc); d != "" {
		parts = append(parts, d)
	}
	return strings.Join(parts, "\n")
}
