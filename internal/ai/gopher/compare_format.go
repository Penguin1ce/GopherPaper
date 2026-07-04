package gopher

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	compareH1Pattern = regexp.MustCompile(`(?m)^#\s+\S`)
	compareSections  = []string{
		"## 方法对比表",
		"## 相同点总结与分析",
		"## 差异分析说明",
		"## 创新、局限与未来方向",
		"## 适用场景与取舍",
	}
	compareTableHeaders = []string{
		"论文",
		"研究问题",
		"方法路线",
		"数据集",
		"评价指标",
		"实验结论",
	}
)

func validateCompareReport(content string, paperCount int) []string {
	content = strings.TrimSpace(content)
	issues := make([]string, 0, 8)
	if !strings.HasPrefix(content, "# 多论文对比分析") {
		issues = append(issues, "一级标题必须为“# 多论文对比分析”")
	}
	if count := len(compareH1Pattern.FindAllString(content, -1)); count != 1 {
		issues = append(issues, "报告必须且只能包含一个一级标题")
	}
	for _, section := range compareSections {
		if !strings.Contains(content, section) {
			issues = append(issues, "缺少固定章节“"+section+"”")
		}
	}

	rows, found := compareTableRows(content)
	if !found {
		issues = append(issues, "缺少固定六列表头的方法对比表")
	} else if len(rows) != paperCount {
		issues = append(issues, fmt.Sprintf("方法对比表应有 %d 篇论文数据行，实际为 %d 行", paperCount, len(rows)))
	}
	if strings.Contains(strings.ToLower(content), "<br") {
		issues = append(issues, "表格和正文不能包含 HTML 换行标签")
	}
	return issues
}

func compareTableRows(content string) ([][]string, bool) {
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		cells := splitCompareTableRow(line)
		if !sameStrings(cells, compareTableHeaders) {
			continue
		}
		if index+1 >= len(lines) || !isMarkdownTableDivider(lines[index+1], len(compareTableHeaders)) {
			return nil, false
		}
		rows := make([][]string, 0)
		for rowIndex := index + 2; rowIndex < len(lines); rowIndex++ {
			row := splitCompareTableRow(lines[rowIndex])
			if len(row) == 0 {
				break
			}
			if len(row) != len(compareTableHeaders) {
				return rows, true
			}
			rows = append(rows, row)
		}
		return rows, true
	}
	return nil, false
}

func splitCompareTableRow(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
		return nil
	}
	line = strings.TrimPrefix(strings.TrimSuffix(line, "|"), "|")
	raw := strings.Split(line, "|")
	cells := make([]string, 0, len(raw))
	for _, cell := range raw {
		cells = append(cells, strings.TrimSpace(cell))
	}
	return cells
}

func isMarkdownTableDivider(line string, columns int) bool {
	cells := splitCompareTableRow(line)
	if len(cells) != columns {
		return false
	}
	for _, cell := range cells {
		cell = strings.Trim(cell, ": ")
		if len(cell) < 3 || strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return true
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
