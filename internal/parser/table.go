package parser

import (
	"strings"

	"golang.org/x/net/html"
)

// tableCell 是表格单元格,携带跨行跨列信息。
type tableCell struct {
	text    string
	colspan int
	rowspan int
}

// tableToMarkdown 把 MinerU table_body 的 HTML 表格转成 GitHub Markdown 表格。
// 处理 colspan/rowspan:把跨格的值平铺到每个被覆盖的格(每列自带表头便于检索与渲染),
// 首行作表头。无法解析或无单元格时返回空串,由调用方回退按图处理。
func tableToMarkdown(tableHTML string) string {
	tableHTML = strings.TrimSpace(tableHTML)
	if tableHTML == "" {
		return ""
	}
	root, err := html.Parse(strings.NewReader(tableHTML))
	if err != nil {
		return ""
	}
	rows := collectRows(root)
	if len(rows) == 0 {
		return ""
	}
	grid := expandGrid(rows)
	if len(grid) == 0 || len(grid[0]) == 0 {
		return ""
	}
	return renderMarkdown(grid)
}

// collectRows 深度遍历取出每个 tr 的单元格序列。
func collectRows(n *html.Node) [][]tableCell {
	var rows [][]tableCell
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "tr" {
			if cells := rowCells(node); len(cells) > 0 {
				rows = append(rows, cells)
			}
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return rows
}

// rowCells 取一个 tr 下的 td/th 单元格,读出文本与跨格数。
func rowCells(tr *html.Node) []tableCell {
	var cells []tableCell
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || (c.Data != "td" && c.Data != "th") {
			continue
		}
		cells = append(cells, tableCell{
			text:    cleanCell(textOf(c)),
			colspan: spanAttr(c, "colspan"),
			rowspan: spanAttr(c, "rowspan"),
		})
	}
	return cells
}

// expandGrid 把带跨格的行展开成规整二维网格,跨格值平铺到每个被覆盖格。
func expandGrid(rows [][]tableCell) [][]string {
	width := 0
	for _, row := range rows {
		w := 0
		for _, c := range row {
			w += c.colspan
		}
		if w > width {
			width = w
		}
	}
	if width == 0 {
		return nil
	}
	grid := make([][]string, len(rows))
	filled := make([][]bool, len(rows))
	for i := range grid {
		grid[i] = make([]string, width)
		filled[i] = make([]bool, width)
	}
	for ri, row := range rows {
		ci := 0
		for _, cell := range row {
			for ci < width && filled[ri][ci] {
				ci++
			}
			for dr := 0; dr < cell.rowspan && ri+dr < len(rows); dr++ {
				for dc := 0; dc < cell.colspan && ci+dc < width; dc++ {
					grid[ri+dr][ci+dc] = cell.text
					filled[ri+dr][ci+dc] = true
				}
			}
			ci += cell.colspan
		}
	}
	return grid
}

// renderMarkdown 把网格渲染为 Markdown 表格,首行表头,其余正文。
func renderMarkdown(grid [][]string) string {
	var b strings.Builder
	writeRow(&b, grid[0])
	sep := make([]string, len(grid[0]))
	for i := range sep {
		sep[i] = "---"
	}
	writeRow(&b, sep)
	for _, row := range grid[1:] {
		writeRow(&b, row)
	}
	return strings.TrimRight(b.String(), "\n")
}

func writeRow(b *strings.Builder, cells []string) {
	b.WriteString("| ")
	b.WriteString(strings.Join(cells, " | "))
	b.WriteString(" |\n")
}

// textOf 收集节点下的全部文本。
func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// cleanCell 归一单元格文本:折叠空白、转义竖线(防破坏 Markdown 表格列)。
func cleanCell(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.ReplaceAll(s, "|", "\\|")
}

// spanAttr 读 colspan/rowspan 属性,缺省或非法回退 1。
func spanAttr(n *html.Node, name string) int {
	for _, a := range n.Attr {
		if a.Key != name {
			continue
		}
		v := 0
		for _, ch := range strings.TrimSpace(a.Val) {
			if ch < '0' || ch > '9' {
				v = 0
				break
			}
			v = v*10 + int(ch-'0')
		}
		if v > 0 {
			return v
		}
	}
	return 1
}
