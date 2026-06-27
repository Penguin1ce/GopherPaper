package paper

import (
	"fmt"
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
		parts := tableChunkParts(tbl, constant.MaxChunkRunes)
		for i, part := range parts {
			if part.content == "" {
				continue
			}
			meta := map[string]any{
				constant.MilvusFieldBlockType: constant.BlockTypeTable,
				"table_part":                  i + 1,
				"table_parts":                 len(parts),
			}
			if part.rowStart > 0 {
				meta["row_start"] = part.rowStart
				meta["row_end"] = part.rowEnd
			}
			content := part.content
			if s := strings.TrimSpace(tbl.SectionPath); s != "" {
				content = s + "\n" + content // 章节语境进 embedding,与正文/图/代码块统一
				meta["section"] = s
			}
			chunks = append(chunks, knowledge.Chunk{
				Content:    content,
				Scope:      constant.KnowledgeScopePrivate,
				OwnerID:    task.OwnerID,
				DocID:      task.PaperID,
				SourceFile: task.FileName,
				PageNo:     int64(tbl.PageNo),
				ChunkIndex: int64(len(chunks)),
				Metadata:   meta,
			})
		}
	}
	return chunks
}

// buildCodeChunks 把算法、伪代码与 prompt 块独立入库,避免混进普通段落后丢失代码边界。
func buildCodeChunks(task parseTask, doc *core.ParsedDoc) []knowledge.Chunk {
	chunks := make([]knowledge.Chunk, 0, len(doc.CodeBlocks))
	for _, code := range doc.CodeBlocks {
		content := codeContent(code)
		if content == "" {
			continue
		}
		chunks = append(chunks, knowledge.Chunk{
			Content:    content,
			Scope:      constant.KnowledgeScopePrivate,
			OwnerID:    task.OwnerID,
			DocID:      task.PaperID,
			SourceFile: task.FileName,
			PageNo:     int64(code.PageNo),
			ChunkIndex: int64(len(chunks)),
			Metadata: map[string]any{
				constant.MilvusFieldBlockType: constant.BlockTypeCode,
				"section":                     code.SectionPath,
				"language":                    code.Language,
			},
		})
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

type tableChunkPart struct {
	content  string
	rowStart int
	rowEnd   int
}

func tableContentParts(tbl core.Table, maxRunes int) []string {
	parts := tableChunkParts(tbl, maxRunes)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, part.content)
	}
	return out
}

func tableChunkParts(tbl core.Table, maxRunes int) []tableChunkPart {
	if len(tbl.Rows) > 0 && len(tbl.Rows[0]) > 0 {
		return tableRowChunkParts(tbl, maxRunes)
	}
	content := tableContent(tbl)
	if content == "" {
		return nil
	}
	if maxRunes <= 0 || runeLen(content) <= maxRunes {
		return []tableChunkPart{{content: content}}
	}
	caption := strings.TrimSpace(tbl.Caption)
	body := strings.TrimSpace(tbl.Markdown)
	if caption == "" || body == "" {
		return textChunkParts(splitLongText(content, maxRunes))
	}
	prefix := caption + "\n"
	bodyLimit := maxRunes - runeLen(prefix)
	if bodyLimit < maxRunes/4 {
		return textChunkParts(splitLongText(content, maxRunes))
	}
	bodyParts := splitLongText(body, bodyLimit)
	out := make([]tableChunkPart, 0, len(bodyParts))
	for _, part := range bodyParts {
		out = append(out, tableChunkPart{content: strings.TrimSpace(prefix + part)})
	}
	return out
}

func textChunkParts(parts []string) []tableChunkPart {
	out := make([]tableChunkPart, 0, len(parts))
	for _, part := range parts {
		out = append(out, tableChunkPart{content: part})
	}
	return out
}

func tableRowChunkParts(tbl core.Table, maxRunes int) []tableChunkPart {
	header := tbl.Rows[0]
	rows := tbl.Rows[1:]
	content := tablePartContent(tbl.Caption, header, rows)
	if content == "" {
		return nil
	}
	if len(rows) == 0 {
		return textChunkParts(splitLongText(content, maxRunes))
	}
	if maxRunes <= 0 || runeLen(content) <= maxRunes {
		return []tableChunkPart{{content: content, rowStart: 1, rowEnd: len(rows)}}
	}
	var parts []tableChunkPart
	var group [][]string
	rowStart := 0
	flush := func() {
		if len(group) == 0 {
			return
		}
		parts = append(parts, tableChunkPart{
			content:  tablePartContent(tbl.Caption, header, group),
			rowStart: rowStart,
			rowEnd:   rowStart + len(group) - 1,
		})
		group = nil
		rowStart = 0
	}
	for i, row := range rows {
		rowNo := i + 1
		single := tablePartContent(tbl.Caption, header, [][]string{row})
		if runeLen(single) > maxRunes {
			flush()
			parts = append(parts, splitLongTableRow(tbl.Caption, header, row, rowNo, maxRunes)...)
			continue
		}
		if len(group) == 0 {
			group = append(group, row)
			rowStart = rowNo
			continue
		}
		candidateRows := append(append([][]string{}, group...), row)
		if runeLen(tablePartContent(tbl.Caption, header, candidateRows)) > maxRunes {
			flush()
			group = append(group, row)
			rowStart = rowNo
			continue
		}
		group = append(group, row)
	}
	flush()
	return parts
}

func tablePartContent(caption string, header []string, rows [][]string) string {
	grid := make([][]string, 0, len(rows)+1)
	grid = append(grid, header)
	grid = append(grid, rows...)
	parts := make([]string, 0, 2)
	if caption = strings.TrimSpace(caption); caption != "" {
		parts = append(parts, caption)
	}
	parts = append(parts, renderMarkdownRows(grid))
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func renderMarkdownRows(rows [][]string) string {
	if len(rows) == 0 || len(rows[0]) == 0 {
		return ""
	}
	var b strings.Builder
	writeMarkdownRow(&b, rows[0])
	sep := make([]string, len(rows[0]))
	for i := range sep {
		sep[i] = "---"
	}
	writeMarkdownRow(&b, sep)
	for _, row := range rows[1:] {
		writeMarkdownRow(&b, normalizeRowWidth(row, len(rows[0])))
	}
	return strings.TrimRight(b.String(), "\n")
}

func writeMarkdownRow(b *strings.Builder, cells []string) {
	b.WriteString("| ")
	b.WriteString(strings.Join(cells, " | "))
	b.WriteString(" |\n")
}

func normalizeRowWidth(row []string, width int) []string {
	out := make([]string, width)
	copy(out, row)
	return out
}

func splitLongTableRow(caption string, header, row []string, rowNo, maxRunes int) []tableChunkPart {
	rowText := tableRowKeyValues(header, row)
	prefix := tableRowPrefix(caption, rowNo)
	if maxRunes <= 0 || runeLen(prefix+rowText) <= maxRunes {
		return []tableChunkPart{{content: strings.TrimSpace(prefix + rowText), rowStart: rowNo, rowEnd: rowNo}}
	}
	var parts []tableChunkPart
	longCellThreshold := maxRunes / 3
	hasLongCell := false
	for _, cell := range row {
		if runeLen(cell) > longCellThreshold {
			hasLongCell = true
			break
		}
	}
	for i, cell := range row {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			continue
		}
		if hasLongCell && runeLen(cell) <= longCellThreshold {
			continue
		}
		col := columnName(header, i)
		context := shortRowContext(header, row, i)
		partPrefix := fmt.Sprintf("%s列 %s", prefix, col)
		if context != "" {
			partPrefix += "\n同一行其他短字段: " + context
		}
		partPrefix += "\n"
		bodyLimit := maxRunes - runeLen(partPrefix)
		if bodyLimit < maxRunes/4 {
			bodyLimit = maxRunes / 2
		}
		pieces := splitLongText(cell, bodyLimit)
		for j, piece := range pieces {
			label := ""
			if len(pieces) > 1 {
				label = fmt.Sprintf("第 %d/%d 段: ", j+1, len(pieces))
			}
			parts = append(parts, tableChunkPart{
				content:  strings.TrimSpace(partPrefix + label + piece),
				rowStart: rowNo,
				rowEnd:   rowNo,
			})
		}
	}
	if len(parts) == 0 {
		return textChunkParts(splitLongText(prefix+rowText, maxRunes))
	}
	return parts
}

func tableRowPrefix(caption string, rowNo int) string {
	if caption = strings.TrimSpace(caption); caption == "" {
		return fmt.Sprintf("表格行 %d\n", rowNo)
	}
	return fmt.Sprintf("%s\n表格行 %d\n", caption, rowNo)
}

func tableRowKeyValues(header, row []string) string {
	lines := make([]string, 0, len(row))
	for i, cell := range row {
		if cell = strings.TrimSpace(cell); cell != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", columnName(header, i), cell))
		}
	}
	return strings.Join(lines, "\n")
}

func shortRowContext(header, row []string, skip int) string {
	var fields []string
	for i, cell := range row {
		cell = strings.TrimSpace(cell)
		if i == skip || cell == "" || runeLen(cell) > 120 {
			continue
		}
		fields = append(fields, fmt.Sprintf("%s=%s", columnName(header, i), cell))
	}
	return strings.Join(fields, "；")
}

func columnName(header []string, i int) string {
	if i < len(header) {
		if name := strings.TrimSpace(header[i]); name != "" {
			return name
		}
	}
	return fmt.Sprintf("第 %d 列", i+1)
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
	parts := make([]string, 0, 3)
	if s := strings.TrimSpace(fig.SectionPath); s != "" {
		parts = append(parts, s) // 章节语境进 embedding,与正文/代码块统一
	}
	if c := strings.TrimSpace(fig.Caption); c != "" {
		parts = append(parts, c)
	}
	if d := strings.TrimSpace(fig.Desc); d != "" {
		parts = append(parts, d)
	}
	return strings.Join(parts, "\n")
}

func codeContent(code core.CodeBlock) string {
	parts := make([]string, 0, 3)
	if s := strings.TrimSpace(code.SectionPath); s != "" {
		parts = append(parts, s)
	}
	if c := strings.TrimSpace(code.Caption); c != "" {
		parts = append(parts, c)
	}
	if b := strings.TrimSpace(code.Body); b != "" {
		parts = append(parts, b)
	}
	return strings.Join(parts, "\n")
}
