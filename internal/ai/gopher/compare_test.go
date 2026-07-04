package gopher

import (
	"strings"
	"testing"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/ai/retrieval"
)

func TestValidateCompareReportAcceptsContract(t *testing.T) {
	report := `# 多论文对比分析

两篇论文关注同一任务，但实验协议不同，因此只能部分比较。

## 方法对比表

| 论文 | 研究问题 | 方法路线 | 数据集 | 评价指标 | 实验结论 |
| --- | --- | --- | --- | --- | --- |
| Paper A | 问题 A | 方法 A | Dataset A | Accuracy | 在给定设置下有效 |
| Paper B | 问题 B | 方法 B | Dataset B | F1 | 在给定设置下有效 |

## 相同点总结与分析

- 都关注目标任务。

## 差异分析说明

### 研究目标
目标不同。

### 技术路线
路线不同。

### 实验设计与可比性
协议不同。

### 结论与证据边界
只能部分比较。

## 创新、局限与未来方向

当前信息有限。

## 适用场景与取舍

按任务约束选择。`

	if issues := validateCompareReport(report, 2); len(issues) != 0 {
		t.Fatalf("validateCompareReport() issues = %v, want none", issues)
	}
}

func TestValidateCompareReportFindsStructuralProblems(t *testing.T) {
	report := `# 错误标题

## 方法对比表

| 论文 | 研究问题 | 方法路线 | 数据集 | 评价指标 | 实验结论 |
| --- | --- | --- | --- | --- | --- |
| Paper A | 问题 A | 方法 A | 未抽取 | 未抽取 | 未抽取<br>结论 |

## 相同点总结与分析

内容`

	issues := validateCompareReport(report, 2)
	joined := strings.Join(issues, "\n")
	for _, want := range []string{
		"一级标题必须为",
		"缺少固定章节“## 差异分析说明”",
		"应有 2 篇论文数据行",
		"不能包含 HTML 换行标签",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("validateCompareReport() issues = %q, want substring %q", joined, want)
		}
	}
}

func TestCompareQueryIncludesPaperIDsAndSkill(t *testing.T) {
	query, err := compareQuery([]core.PaperCompareInput{
		{ID: "paper-a", Title: "Paper A"},
		{ID: "paper-b", Title: "Paper B"},
	})
	if err != nil {
		t.Fatalf("compareQuery() error = %v", err)
	}
	for _, want := range []string{"$report-compare", "paper-a", "paper-b", "search_compare_paper"} {
		if !strings.Contains(query, want) {
			t.Errorf("compareQuery() missing %q", want)
		}
	}
}

func TestCompareScopesUsesStableFallbackNames(t *testing.T) {
	scopes := compareScopes([]core.PaperCompareInput{
		{ID: "a", Title: "Title A", FileName: "a.pdf"},
		{ID: "b", FileName: "b.pdf"},
		{ID: "c"},
	})
	got := []string{scopes[0].Title, scopes[1].Title, scopes[2].Title}
	want := []string{"Title A", "b.pdf", "c"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("compareScopes()[%d].Title = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestCompareReplyMetaCountsEvidenceByPaper(t *testing.T) {
	meta := compareReplyMeta(
		[]core.PaperCompareInput{{ID: "a"}, {ID: "b"}},
		[]retrieval.Reference{
			{ID: "a-1", DocID: "a"},
			{ID: "a-2", DocID: "a"},
			{ID: "b-1", DocID: "b"},
		},
		"agentic",
	)
	counts, ok := meta["source_counts"].(map[string]int)
	if !ok {
		t.Fatalf("source_counts type = %T", meta["source_counts"])
	}
	if counts["a"] != 2 || counts["b"] != 1 {
		t.Fatalf("source_counts = %v", counts)
	}
	if meta["source_count"] != 3 {
		t.Fatalf("source_count = %v, want 3", meta["source_count"])
	}
}
