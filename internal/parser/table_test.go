package parser

import "testing"

func TestTableToMarkdownSimple(t *testing.T) {
	html := `<table><tr><td>Layer Type</td><td>Complexity</td></tr>` +
		`<tr><td>Self-Attention</td><td> $O(n^{2})$ </td></tr></table>`
	got := tableToMarkdown(html)
	want := "| Layer Type | Complexity |\n| --- | --- |\n| Self-Attention | $O(n^{2})$ |"
	if got != want {
		t.Fatalf("简单表格转换不符:\n got=%q\nwant=%q", got, want)
	}
}

func TestTableToMarkdownSpan(t *testing.T) {
	// colspan 表头平铺到每列,rowspan 纵向填充。
	html := `<table>` +
		`<tr><td rowspan="2">Model</td><td colspan="2">BLEU</td></tr>` +
		`<tr><td>EN-DE</td><td>EN-FR</td></tr>` +
		`<tr><td>X</td><td>1</td><td>2</td></tr>` +
		`</table>`
	got := tableToMarkdown(html)
	want := "| Model | BLEU | BLEU |\n| --- | --- | --- |\n| Model | EN-DE | EN-FR |\n| X | 1 | 2 |"
	if got != want {
		t.Fatalf("跨格表格转换不符:\n got=%q\nwant=%q", got, want)
	}
}

func TestTableToMarkdownPipeEscape(t *testing.T) {
	html := `<table><tr><td>a|b</td><td>c</td></tr></table>`
	got := tableToMarkdown(html)
	want := "| a\\|b | c |\n| --- | --- |"
	if got != want {
		t.Fatalf("竖线转义不符:\n got=%q\nwant=%q", got, want)
	}
}

func TestTableToMarkdownEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "<div>not a table</div>", "<table></table>"} {
		if got := tableToMarkdown(in); got != "" {
			t.Fatalf("非表格/空输入应返回空,得到 %q (输入 %q)", got, in)
		}
	}
}
