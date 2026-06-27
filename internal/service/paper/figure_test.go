package paper

import (
	"strings"
	"testing"

	"GopherPaper/internal/ai/core"
)

func TestTableChunkPartsSplitsLongRowsWithColumnContext(t *testing.T) {
	longText := strings.Repeat("Original AI Text. Simple Paraphrase. Adversarial Paraphrase. ", 20)
	tbl := core.Table{
		Caption: "Table 14: Examples of original AI texts with paraphrases.",
		Rows: [][]string{
			{"Text", "Rating"},
			{longText, "-55"},
		},
	}

	parts := tableChunkParts(tbl, 320)
	if len(parts) < 2 {
		t.Fatalf("超长表格行应被拆成多段,得到 %d 段: %+v", len(parts), parts)
	}
	for _, part := range parts {
		if !strings.Contains(part.content, tbl.Caption) {
			t.Fatalf("每段都应保留表题上下文: %q", part.content)
		}
		if !strings.Contains(part.content, "表格行 1") || !strings.Contains(part.content, "列 Text") {
			t.Fatalf("长行分段应保留行号与列名: %q", part.content)
		}
		if strings.Contains(part.content, "| "+longText[:40]) {
			t.Fatalf("超长行不应退回半截 Markdown 表格: %q", part.content)
		}
		if part.rowStart != 1 || part.rowEnd != 1 {
			t.Fatalf("行范围元数据错误: %+v", part)
		}
	}
	if !strings.Contains(parts[0].content, "Rating=-55") {
		t.Fatalf("长文本列应携带同一行短字段上下文: %q", parts[0].content)
	}
}

func TestTableChunkPartsKeepsCompactRowsAsMarkdown(t *testing.T) {
	tbl := core.Table{
		Caption: "Table 3: Runtime.",
		Rows: [][]string{
			{"Method", "Run time"},
			{"Simple", "7.18"},
			{"AdvPara", "10.20"},
		},
	}

	parts := tableChunkParts(tbl, 1000)
	if len(parts) != 1 {
		t.Fatalf("短表格不应被拆分,得到 %d 段", len(parts))
	}
	if !strings.Contains(parts[0].content, "| Method | Run time |") || !strings.Contains(parts[0].content, "| AdvPara | 10.20 |") {
		t.Fatalf("短表格应保持 Markdown 形态: %q", parts[0].content)
	}
}
