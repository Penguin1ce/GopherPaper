// Package parser 把 PDF 经 MinerU 在线 API 解析为结构化 ParsedDoc。
// 只在 parse worker 的异步链路调用：流程含分钟级轮询，不进 HTTP 请求主链路。
// 以 content_list_v2.json 为准解析(跨 MinerU 版本 schema 稳定),旧版扁平 content_list.json
// 仅作缺 v2 时的回退。MinerU 细节在本包内消化，对外只暴露 core.ParsedDoc。
package parser

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"GopherPaper/internal/ai/core"
)

// contentBlock 是本包内归一化的内容块:由 content_list_v2.json 转换而来(convertV2Blocks),
// 或直读旧版 content_list.json,下游 mapBlocks 只认这一种结构。
type contentBlock struct {
	Type          string     `json:"type"` // text/title/image/table/equation
	Text          string     `json:"text"`
	TextLevel     int        `json:"text_level"` // 标题层级，正文为 0
	PageIdx       int        `json:"page_idx"`   // 0 起页码
	ImgPath       string     `json:"img_path"`   // 产物 zip 内图片相对路径，如 images/xxx.jpg
	ImgCaption    stringList `json:"img_caption"`
	ImageCaption  stringList `json:"image_caption"`
	ImageFootnote stringList `json:"image_footnote"`
	TableCaption  stringList `json:"table_caption"`
	TableFootnote stringList `json:"table_footnote"`
	TableBody     string     `json:"table_body"` // 表格 HTML,MinerU 已结构化识别,转 Markdown 走文本不返图
	ChartCaption  stringList `json:"chart_caption"`
	ChartFootnote stringList `json:"chart_footnote"`
	CodeBody      string     `json:"code_body"`
	CodeCaption   stringList `json:"code_caption"`
	CodeLanguage  string     `json:"code_language"`
	SubType       string     `json:"sub_type"`
}

type stringList []string

func (s *stringList) UnmarshalJSON(data []byte) error {
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*s = compactList(arr)
		return nil
	}
	var one string
	if err := json.Unmarshal(data, &one); err == nil {
		*s = compactList([]string{one})
		return nil
	}
	*s = nil
	return nil
}

type detailDoc struct {
	PDFInfo []detailPage `json:"pdf_info"`
}

type detailPage struct {
	ParaBlocks []detailParaBlock `json:"para_blocks"`
}

type detailParaBlock struct {
	Blocks []detailBlock `json:"blocks"`
}

type detailBlock struct {
	Type  string       `json:"type"`
	Lines []detailLine `json:"lines"`
}

type detailLine struct {
	Spans []detailSpan `json:"spans"`
}

type detailSpan struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type contentListV2Block struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content"`
}

type v2Inline struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type v2ImageSource struct {
	Path string `json:"path"`
}

type v2ListItem struct {
	ItemContent []v2Inline `json:"item_content"`
}

// imgExtRe 匹配产物 zip 里的图片文件,连同 content_list 一并取出供带图问答。
var imgExtRe = regexp.MustCompile(`(?i)\.(jpe?g|png|gif|webp|bmp)$`)

var refTitleRe = regexp.MustCompile(`(?i)^\s*(references|bibliography|参考文献)\s*$`)

// Parse 把本地 PDF 解析为 ParsedDoc。
func Parse(ctx context.Context, fileURI string) (*core.ParsedDoc, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("parser: 未初始化")
	}
	data, err := os.ReadFile(fileURI)
	if err != nil {
		return nil, fmt.Errorf("parser: 读取文件失败: %w", err)
	}

	batchID, putURL, err := requestUpload(ctx, baseName(fileURI))
	if err != nil {
		return nil, err
	}
	if err := uploadFile(ctx, putURL, data); err != nil {
		return nil, err
	}
	zipURL, err := pollBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	zipData, err := downloadZip(ctx, zipURL)
	if err != nil {
		return nil, err
	}
	blocks, images, detailRefs, err := readArtifacts(zipData)
	if err != nil {
		return nil, err
	}
	doc := mapBlocks(blocks, images)
	doc.References = mergeReferences(doc.References, detailRefs)
	doc.Artifact = zipData // 原始产物随 doc 带回,供 worker 解压归档为重建铺垫
	return doc, nil
}

// ParseArtifactDir 从已归档的 MinerU 产物目录重建 ParsedDoc,不重新调用 MinerU 在线 API。
func ParseArtifactDir(dir string) (*core.ParsedDoc, error) {
	blocks, images, detailRefs, err := readArtifactDir(dir)
	if err != nil {
		return nil, err
	}
	doc := mapBlocks(blocks, images)
	doc.References = mergeReferences(doc.References, detailRefs)
	return doc, nil
}

// readArtifacts 从产物 zip 里取出内容块(优先 content_list_v2.json,回退 content_list.json)、
// 细粒度 ref_text 兜底引用与全部图片字节。
func readArtifacts(zipData []byte) ([]contentBlock, map[string][]byte, []string, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parser: 解压产物失败: %w", err)
	}
	var blocksV1 []contentBlock
	var blocksV2 []contentBlock
	var refs []string
	foundV1 := false
	foundV2 := false
	images := map[string][]byte{}
	for _, f := range zr.File {
		switch {
		case strings.HasSuffix(f.Name, "content_list_v2.json"):
			raw, err := readZipFile(f)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("parser: 读取 content_list_v2 失败: %w", err)
			}
			var pages [][]contentListV2Block
			if err := json.Unmarshal(raw, &pages); err != nil {
				return nil, nil, nil, fmt.Errorf("parser: 解析 content_list_v2 失败: %w", err)
			}
			blocksV2 = convertV2Blocks(pages)
			foundV2 = true
		case strings.HasSuffix(f.Name, "content_list.json"):
			raw, err := readZipFile(f)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("parser: 读取 content_list 失败: %w", err)
			}
			if err := json.Unmarshal(raw, &blocksV1); err != nil {
				return nil, nil, nil, fmt.Errorf("parser: 解析 content_list 失败: %w", err)
			}
			foundV1 = true
		case isDetailJSON(f.Name):
			raw, err := readZipFile(f)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("parser: 读取细粒度 JSON 失败: %w", err)
			}
			refs = append(refs, extractDetailRefs(raw)...)
		case imgExtRe.MatchString(f.Name):
			raw, err := readZipFile(f)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("parser: 读取图片失败: %w", err)
			}
			images[baseName(f.Name)] = raw
		}
	}
	if foundV2 && len(blocksV2) > 0 {
		return blocksV2, images, refs, nil
	}
	if foundV1 {
		return blocksV1, images, refs, nil
	}
	if foundV2 {
		return blocksV2, images, refs, nil
	}
	return nil, nil, nil, fmt.Errorf("parser: 产物中未找到 content_list.json")
}

func readArtifactDir(dir string) ([]contentBlock, map[string][]byte, []string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parser: 读取 MinerU 归档目录失败: %w", err)
	}
	if !info.IsDir() {
		return nil, nil, nil, fmt.Errorf("parser: MinerU 归档路径不是目录: %s", dir)
	}
	var blocksV1 []contentBlock
	var blocksV2 []contentBlock
	var refs []string
	foundV1 := false
	foundV2 := false
	images := map[string][]byte{}
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		name := filepath.ToSlash(path)
		switch {
		case strings.HasSuffix(name, "content_list_v2.json"):
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("parser: 读取 content_list_v2 失败: %w", err)
			}
			var pages [][]contentListV2Block
			if err := json.Unmarshal(raw, &pages); err != nil {
				return fmt.Errorf("parser: 解析 content_list_v2 失败: %w", err)
			}
			blocksV2 = convertV2Blocks(pages)
			foundV2 = true
		case strings.HasSuffix(name, "content_list.json"):
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("parser: 读取 content_list 失败: %w", err)
			}
			if err := json.Unmarshal(raw, &blocksV1); err != nil {
				return fmt.Errorf("parser: 解析 content_list 失败: %w", err)
			}
			foundV1 = true
		case isDetailJSON(name):
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("parser: 读取细粒度 JSON 失败: %w", err)
			}
			refs = append(refs, extractDetailRefs(raw)...)
		case imgExtRe.MatchString(name):
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("parser: 读取图片失败: %w", err)
			}
			images[baseName(path)] = raw
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, err
	}
	if foundV2 && len(blocksV2) > 0 {
		return blocksV2, images, refs, nil
	}
	if foundV1 {
		return blocksV1, images, refs, nil
	}
	if foundV2 {
		return blocksV2, images, refs, nil
	}
	return nil, nil, nil, fmt.Errorf("parser: 归档目录中未找到 content_list.json")
}

// readZipFile 读出 zip 内单个文件的全部字节。
func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func convertV2Blocks(pages [][]contentListV2Block) []contentBlock {
	var blocks []contentBlock
	for pageIdx, page := range pages {
		for _, b := range page {
			content := v2ContentMap(b.Content)
			switch b.Type {
			case "title":
				blocks = append(blocks, contentBlock{
					Type:      "title",
					Text:      v2InlineTextField(content, "title_content", false),
					TextLevel: v2IntField(content, "level"),
					PageIdx:   pageIdx,
				})
			case "paragraph":
				blocks = append(blocks, contentBlock{
					Type:    "text",
					Text:    v2InlineTextField(content, "paragraph_content", false),
					PageIdx: pageIdx,
				})
			case "equation_interline":
				// 独立编号公式,以 $$...$$ 包好交给 mapBlocks 的 equation 分支,前端 KaTeX 渲染。
				if math := v2StringField(content, "math_content"); math != "" {
					blocks = append(blocks, contentBlock{
						Type:    "equation",
						Text:    "$$" + math + "$$",
						PageIdx: pageIdx,
					})
				}
			case "list":
				items := v2ListItemTexts(content)
				if v2StringField(content, "list_type") == "reference_list" {
					for _, item := range items {
						blocks = append(blocks, contentBlock{Type: "ref_text", Text: item, PageIdx: pageIdx})
					}
					continue
				}
				blocks = append(blocks, contentBlock{
					Type:    "text",
					Text:    strings.Join(items, "\n"),
					PageIdx: pageIdx,
				})
			case "table":
				blocks = append(blocks, contentBlock{
					Type:          "table",
					TableBody:     v2StringField(content, "html"),
					TableCaption:  v2InlineListField(content, "table_caption"),
					TableFootnote: v2InlineListField(content, "table_footnote"),
					ImgPath:       v2ImageSourcePath(content),
					PageIdx:       pageIdx,
				})
			case "chart":
				blocks = append(blocks, contentBlock{
					Type:          "chart",
					ChartCaption:  v2InlineListField(content, "chart_caption"),
					ChartFootnote: v2InlineListField(content, "chart_footnote"),
					ImgPath:       v2ImageSourcePath(content),
					PageIdx:       pageIdx,
				})
			case "image":
				blocks = append(blocks, contentBlock{
					Type:          "image",
					ImageCaption:  v2InlineListField(content, "image_caption"),
					ImageFootnote: v2InlineListField(content, "image_footnote"),
					ImgPath:       v2ImageSourcePath(content),
					PageIdx:       pageIdx,
				})
			case "code":
				blocks = append(blocks, contentBlock{
					Type:         "code",
					CodeBody:     v2InlineTextField(content, "code_content", true),
					CodeCaption:  v2InlineListField(content, "code_caption"),
					CodeLanguage: v2StringField(content, "code_language"),
					PageIdx:      pageIdx,
				})
			case "page_footnote":
				blocks = append(blocks, contentBlock{
					Type:    "page_footnote",
					Text:    v2InlineTextField(content, "page_footnote_content", false),
					PageIdx: pageIdx,
				})
			}
			// page_header/page_footer/page_number/page_aside_text 为页眉页脚水印噪声,不入正文。
		}
	}
	return blocks
}

func v2ContentMap(raw json.RawMessage) map[string]json.RawMessage {
	var out map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &out) != nil {
		return map[string]json.RawMessage{}
	}
	return out
}

func v2InlineTextField(m map[string]json.RawMessage, key string, preserveSpace bool) string {
	var items []v2Inline
	if err := json.Unmarshal(m[key], &items); err != nil {
		return ""
	}
	return v2InlineText(items, preserveSpace)
}

func v2InlineListField(m map[string]json.RawMessage, key string) stringList {
	if text := v2InlineTextField(m, key, false); text != "" {
		return stringList{text}
	}
	return nil
}

func v2InlineText(items []v2Inline, preserveSpace bool) string {
	var b strings.Builder
	for _, item := range items {
		// 行内公式以 $...$ 包好供 KaTeX 渲染,其余按原文拼接。
		if item.Type == "equation_inline" {
			if content := strings.TrimSpace(item.Content); content != "" {
				b.WriteString(" $")
				b.WriteString(content)
				b.WriteString("$ ")
			}
			continue
		}
		b.WriteString(item.Content)
	}
	text := b.String()
	if preserveSpace {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}

func v2StringField(m map[string]json.RawMessage, key string) string {
	var s string
	if err := json.Unmarshal(m[key], &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func v2IntField(m map[string]json.RawMessage, key string) int {
	var n int
	if err := json.Unmarshal(m[key], &n); err == nil {
		return n
	}
	return 0
}

func v2ImageSourcePath(m map[string]json.RawMessage) string {
	var src v2ImageSource
	if err := json.Unmarshal(m["image_source"], &src); err != nil {
		return ""
	}
	return strings.TrimSpace(src.Path)
}

func v2ListItemTexts(m map[string]json.RawMessage) []string {
	var items []v2ListItem
	if err := json.Unmarshal(m["list_items"], &items); err != nil {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text := v2InlineText(item.ItemContent, false); text != "" {
			out = append(out, text)
		}
	}
	return out
}

// mapBlocks 把 MinerU 块流映射为 ParsedDoc，维护章节路径栈并分流参考文献。
// images 为 zip 内图片字节(键为文件名),按块的 img_path 关联到对应 Figure。
func mapBlocks(blocks []contentBlock, images map[string][]byte) *core.ParsedDoc {
	doc := &core.ParsedDoc{}
	var sectionStack []string // 按层级维护当前标题链
	inReferences := false
	maxPage := 0
	order := 0

	for _, b := range blocks {
		page := b.PageIdx + 1
		if page > maxPage {
			maxPage = page
		}
		switch {
		case b.Type == "title" || (b.Type == "text" && b.TextLevel > 0):
			level := b.TextLevel
			if level <= 0 {
				level = 1
			}
			// 截断到上一层级,再压入当前标题。
			if level-1 < len(sectionStack) {
				sectionStack = sectionStack[:level-1]
			}
			for len(sectionStack) < level-1 {
				sectionStack = append(sectionStack, "")
			}
			sectionStack = append(sectionStack, b.Text)
			inReferences = refTitleRe.MatchString(b.Text)
			doc.Sections = append(doc.Sections, core.Section{
				Level:    level,
				Title:    b.Text,
				PageNo:   page,
				OrderIdx: order,
			})
		case b.Type == "text":
			if strings.TrimSpace(b.Text) == "" {
				break
			}
			if inReferences {
				doc.References = append(doc.References, b.Text)
				break
			}
			doc.Paragraphs = append(doc.Paragraphs, core.Paragraph{
				Text:        b.Text,
				PageNo:      page,
				SectionPath: strings.Join(nonEmpty(sectionStack), " / "),
			})
		case b.Type == "equation":
			// 独立编号公式块(text_format=latex,text 形如 $$...\tag{1}$$):
			// 当正文段落入库,与相邻正文同章节同页便由 buildChunks 合并进同一块,
			// 公式连同解释它的上下文一起向量化召回,前端 KaTeX 渲染。参考文献区内的跳过。
			if text := strings.TrimSpace(b.Text); text != "" && !inReferences {
				doc.Paragraphs = append(doc.Paragraphs, core.Paragraph{
					Text:        text,
					PageNo:      page,
					SectionPath: strings.Join(nonEmpty(sectionStack), " / "),
				})
			}
		case b.Type == "page_footnote":
			if text := strings.TrimSpace(b.Text); text != "" && !inReferences {
				doc.Paragraphs = append(doc.Paragraphs, core.Paragraph{
					Text:        "脚注: " + text,
					PageNo:      page,
					SectionPath: strings.Join(nonEmpty(sectionStack), " / "),
				})
			}
		case b.Type == "ref_text":
			if text := strings.TrimSpace(b.Text); text != "" {
				doc.References = append(doc.References, text)
			}
		case b.Type == "code":
			if body := strings.TrimSpace(b.CodeBody); body != "" && !inReferences {
				doc.CodeBlocks = append(doc.CodeBlocks, core.CodeBlock{
					Caption:     strings.TrimSpace(strings.Join(nonEmpty(b.CodeCaption), " ")),
					Body:        body,
					Language:    strings.TrimSpace(firstNonEmpty(b.CodeLanguage, b.SubType)),
					PageNo:      page,
					SectionPath: strings.Join(nonEmpty(sectionStack), " / "),
				})
			}
		case b.Type == "table":
			caption := blockCaption(b)
			section := strings.Join(nonEmpty(sectionStack), " / ")
			// MinerU 已把表格结构化成 table_body,转 Markdown 走文本入库不返图;
			// 转换失败(空 body 或解析不出)再退化按图处理。
			if grid := tableToGrid(b.TableBody); len(grid) > 0 && len(grid[0]) > 0 {
				tbl := core.Table{
					Caption:     caption,
					Markdown:    renderMarkdown(grid),
					PageNo:      page,
					SectionPath: section,
					ImgPath:     b.ImgPath,
					Rows:        grid,
				}
				if b.ImgPath != "" {
					tbl.ImgData = images[baseName(b.ImgPath)]
					if fig := figureFromBlock(b, caption, page, section, images); fig.Caption != "" || len(fig.ImgData) > 0 {
						doc.Figures = append(doc.Figures, fig)
					}
				}
				doc.Tables = append(doc.Tables, tbl)
				break
			}
			fallthrough
		case b.Type == "image" || b.Type == "chart":
			caption := blockCaption(b)
			fig := figureFromBlock(b, caption, page, strings.Join(nonEmpty(sectionStack), " / "), images)
			// 既无说明也无图片字节的块无从召回,跳过。
			if caption == "" && len(fig.ImgData) == 0 {
				break
			}
			doc.Figures = append(doc.Figures, fig)
		}
		order++
	}
	doc.PageCount = maxPage
	return doc
}

func isDetailJSON(name string) bool {
	return strings.HasSuffix(name, "model.json") || strings.HasSuffix(name, "middle.json") || strings.HasSuffix(name, "layout.json")
}

func extractDetailRefs(raw []byte) []string {
	var doc detailDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	var refs []string
	for _, info := range doc.PDFInfo {
		for _, para := range info.ParaBlocks {
			for _, block := range para.Blocks {
				if block.Type != "ref_text" {
					continue
				}
				if ref := linesText(block.Lines); ref != "" {
					refs = append(refs, ref)
				}
			}
		}
	}
	return refs
}

func linesText(lines []detailLine) string {
	var parts []string
	for _, line := range lines {
		for _, span := range line.Spans {
			if span.Type != "" && span.Type != "text" {
				continue
			}
			if s := strings.TrimSpace(span.Content); s != "" {
				parts = append(parts, s)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func blockCaption(b contentBlock) string {
	captions := make([]string, 0, len(b.ImgCaption)+len(b.ImageCaption)+len(b.ImageFootnote)+len(b.TableCaption)+len(b.TableFootnote)+len(b.ChartCaption)+len(b.ChartFootnote))
	captions = append(captions, b.ImgCaption...)
	captions = append(captions, b.ImageCaption...)
	captions = append(captions, b.ImageFootnote...)
	captions = append(captions, b.TableCaption...)
	captions = append(captions, b.TableFootnote...)
	captions = append(captions, b.ChartCaption...)
	captions = append(captions, b.ChartFootnote...)
	return strings.TrimSpace(strings.Join(nonEmpty(captions), " "))
}

func figureFromBlock(b contentBlock, caption string, page int, section string, images map[string][]byte) core.Figure {
	fig := core.Figure{Caption: caption, PageNo: page, SectionPath: section, ImgPath: b.ImgPath}
	if b.ImgPath != "" {
		fig.ImgData = images[baseName(b.ImgPath)]
	}
	return fig
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mergeReferences(a, b []string) []string {
	out := make([]string, 0, len(a)+len(b))
	seen := map[string]bool{}
	for _, ref := range append(append([]string{}, a...), b...) {
		ref = strings.TrimSpace(ref)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out
}

func nonEmpty(ss []string) []string {
	return compactList(ss)
}

func compactList(ss []string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}
