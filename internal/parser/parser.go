// Package parser 把 PDF 经 MinerU 在线 API 解析为结构化 ParsedDoc。
// 只在 parse worker 的异步链路调用：流程含分钟级轮询，不进 HTTP 请求主链路。
// MinerU 的 content_list.json 细节在本包内消化，对外只暴露 core.ParsedDoc。
package parser

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"GopherPaper/internal/ai/core"
)

// contentBlock 是 MinerU content_list.json 里的一个块。
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

// readArtifacts 从产物 zip 里取出 content_list.json、细粒度 ref_text 与全部图片字节。
func readArtifacts(zipData []byte) ([]contentBlock, map[string][]byte, []string, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parser: 解压产物失败: %w", err)
	}
	var blocks []contentBlock
	var refs []string
	found := false
	images := map[string][]byte{}
	for _, f := range zr.File {
		switch {
		case strings.HasSuffix(f.Name, "content_list.json"):
			raw, err := readZipFile(f)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("parser: 读取 content_list 失败: %w", err)
			}
			if err := json.Unmarshal(raw, &blocks); err != nil {
				return nil, nil, nil, fmt.Errorf("parser: 解析 content_list 失败: %w", err)
			}
			found = true
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
	if !found {
		return nil, nil, nil, fmt.Errorf("parser: 产物中未找到 content_list.json")
	}
	return blocks, images, refs, nil
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
		case b.Type == "ref_text":
			if text := strings.TrimSpace(b.Text); text != "" {
				doc.References = append(doc.References, text)
			}
		case b.Type == "table":
			caption := blockCaption(b)
			// MinerU 已把表格结构化成 table_body,转 Markdown 走文本入库不返图;
			// 转换失败(空 body 或解析不出)再退化按图处理。
			if md := tableToMarkdown(b.TableBody); md != "" {
				doc.Tables = append(doc.Tables, core.Table{Caption: caption, Markdown: md, PageNo: page})
				break
			}
			fallthrough
		case b.Type == "image":
			caption := blockCaption(b)
			fig := core.Figure{Caption: caption, PageNo: page, ImgPath: b.ImgPath}
			if b.ImgPath != "" {
				fig.ImgData = images[baseName(b.ImgPath)]
			}
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
	return strings.HasSuffix(name, "model.json") || strings.HasSuffix(name, "middle.json")
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
	captions := make([]string, 0, len(b.ImgCaption)+len(b.ImageCaption)+len(b.ImageFootnote)+len(b.TableCaption)+len(b.TableFootnote))
	captions = append(captions, b.ImgCaption...)
	captions = append(captions, b.ImageCaption...)
	captions = append(captions, b.ImageFootnote...)
	captions = append(captions, b.TableCaption...)
	captions = append(captions, b.TableFootnote...)
	return strings.TrimSpace(strings.Join(nonEmpty(captions), " "))
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
