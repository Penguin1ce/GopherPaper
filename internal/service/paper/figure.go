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

// buildTableChunks 把表格建成表格块入库:caption + Markdown 表格做向量化文本,
// metadata 标 block_type=table 且不带 img_uri——表格走正文检索以文本返回,不返图、不喂 VLM。
// caption 与表体都空的表无从召回,跳过。
func buildTableChunks(task parseTask, doc *core.ParsedDoc) []knowledge.Chunk {
	chunks := make([]knowledge.Chunk, 0, len(doc.Tables))
	for _, tbl := range doc.Tables {
		parts := tableContentParts(tbl, constant.MaxChunkRunes)
		for i, content := range parts {
			if content == "" {
				continue
			}
			chunks = append(chunks, knowledge.Chunk{
				Content:    content,
				Scope:      constant.KnowledgeScopePrivate,
				OwnerID:    task.OwnerID,
				DocID:      task.PaperID,
				SourceFile: task.FileName,
				PageNo:     int64(tbl.PageNo),
				ChunkIndex: int64(len(chunks)),
				Metadata: map[string]any{
					constant.MilvusFieldBlockType: constant.BlockTypeTable,
					"table_part":                  i + 1,
					"table_parts":                 len(parts),
				},
			})
		}
	}
	return chunks
}

// tableContent 把 caption 与 Markdown 表体拼成表格块向量化文本,两者去空合并。
func tableContent(tbl core.Table) string {
	parts := make([]string, 0, 2)
	if c := strings.TrimSpace(tbl.Caption); c != "" {
		parts = append(parts, c)
	}
	if m := strings.TrimSpace(tbl.Markdown); m != "" {
		parts = append(parts, m)
	}
	return strings.Join(parts, "\n")
}

func tableContentParts(tbl core.Table, maxRunes int) []string {
	content := tableContent(tbl)
	if content == "" {
		return nil
	}
	if maxRunes <= 0 || runeLen(content) <= maxRunes {
		return []string{content}
	}
	caption := strings.TrimSpace(tbl.Caption)
	body := strings.TrimSpace(tbl.Markdown)
	if caption == "" || body == "" {
		return splitLongText(content, maxRunes)
	}
	prefix := caption + "\n"
	bodyLimit := maxRunes - runeLen(prefix)
	if bodyLimit < maxRunes/4 {
		return splitLongText(content, maxRunes)
	}
	bodyParts := splitLongText(body, bodyLimit)
	out := make([]string, 0, len(bodyParts))
	for _, part := range bodyParts {
		out = append(out, strings.TrimSpace(prefix+part))
	}
	return out
}

func splitLongText(text string, maxRunes int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxRunes <= 0 || runeLen(text) <= maxRunes {
		return []string{text}
	}
	var parts []string
	var b strings.Builder
	curRunes := 0
	flush := func() {
		part := strings.TrimSpace(b.String())
		if part != "" {
			parts = append(parts, part)
		}
		b.Reset()
		curRunes = 0
	}
	for _, line := range strings.Split(text, "\n") {
		lineRunes := runeLen(line)
		if lineRunes > maxRunes {
			flush()
			parts = append(parts, splitRunes(line, maxRunes)...)
			continue
		}
		sepRunes := 0
		if curRunes > 0 {
			sepRunes = 1
		}
		if curRunes > 0 && curRunes+sepRunes+lineRunes > maxRunes {
			flush()
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
			curRunes++
		}
		b.WriteString(line)
		curRunes += lineRunes
	}
	flush()
	return parts
}

func splitRunes(text string, maxRunes int) []string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}
	parts := make([]string, 0, (len(runes)+maxRunes-1)/maxRunes)
	for len(runes) > 0 {
		n := maxRunes
		if len(runes) < n {
			n = len(runes)
		}
		parts = append(parts, string(runes[:n]))
		runes = runes[n:]
	}
	return parts
}

func runeLen(text string) int {
	return len([]rune(text))
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
