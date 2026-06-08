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
	Type         string   `json:"type"` // text/title/image/table/equation
	Text         string   `json:"text"`
	TextLevel    int      `json:"text_level"` // 标题层级，正文为 0
	PageIdx      int      `json:"page_idx"`   // 0 起页码
	ImgPath      string   `json:"img_path"`   // 产物 zip 内图片相对路径，如 images/xxx.jpg
	ImgCaption   []string `json:"img_caption"`
	TableCaption []string `json:"table_caption"`
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
	blocks, images, err := readArtifacts(zipData)
	if err != nil {
		return nil, err
	}
	return mapBlocks(blocks, images), nil
}

// readArtifacts 从产物 zip 里取出 content_list.json 与全部图片字节(键为图片文件名)。
func readArtifacts(zipData []byte) ([]contentBlock, map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, nil, fmt.Errorf("parser: 解压产物失败: %w", err)
	}
	var blocks []contentBlock
	found := false
	images := map[string][]byte{}
	for _, f := range zr.File {
		switch {
		case strings.HasSuffix(f.Name, "content_list.json"):
			raw, err := readZipFile(f)
			if err != nil {
				return nil, nil, fmt.Errorf("parser: 读取 content_list 失败: %w", err)
			}
			if err := json.Unmarshal(raw, &blocks); err != nil {
				return nil, nil, fmt.Errorf("parser: 解析 content_list 失败: %w", err)
			}
			found = true
		case imgExtRe.MatchString(f.Name):
			raw, err := readZipFile(f)
			if err != nil {
				return nil, nil, fmt.Errorf("parser: 读取图片失败: %w", err)
			}
			images[baseName(f.Name)] = raw
		}
	}
	if !found {
		return nil, nil, fmt.Errorf("parser: 产物中未找到 content_list.json")
	}
	return blocks, images, nil
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
		switch b.Type {
		case "title":
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
		case "text":
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
		case "image", "table":
			caption := strings.TrimSpace(strings.Join(append(b.ImgCaption, b.TableCaption...), " "))
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

func nonEmpty(ss []string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
