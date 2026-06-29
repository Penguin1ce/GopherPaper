package gopher

import (
	"strings"
	"testing"

	"GopherPaper/pkg/constant"
)

// 报告 prompt 必须带 {focus} 占位符,Generate 据此按报告类型注入聚焦点。
func TestReportPromptHasFocusPlaceholder(t *testing.T) {
	if !strings.Contains(constant.GopherReportPrompt, "{focus}") {
		t.Fatal("GopherReportPrompt 缺少 {focus} 占位符,Generate 无法注入报告聚焦点")
	}
	// 注入后不应残留占位符,且聚焦点文本在位。
	out := strings.ReplaceAll(constant.GopherReportPrompt, "{focus}", constant.ReportMethodFocus)
	if strings.Contains(out, "{focus}") || !strings.Contains(out, constant.ReportMethodFocus) {
		t.Fatalf("focus 注入异常: %q", out)
	}
}

// 空 userID 必须报错,不能懒建出无主 runner。
func TestRunnerForUserRejectsEmpty(t *testing.T) {
	if _, err := runnerForUser(""); err == nil {
		t.Fatal("runnerForUser(\"\") 应返回错误")
	}
}

// 模型跑飞重写第二份报告时,从第二个一级标题处截断,只保留第一份。
// 标题之前残留的思维链自语/垃圾串交由采样参数治理,不在此正则范围。
func TestSanitizeReportCutsDuplicate(t *testing.T) {
	in := "# 报告一\n\n## 背景\n正文内容。\n\n# 报告二\n\n## 背景\n第二份正文。"
	out := sanitizeReport(in)
	if strings.Contains(out, "报告二") || strings.Contains(out, "第二份正文") {
		t.Fatalf("第二份报告未截掉: %q", out)
	}
	if !strings.HasPrefix(out, "# 报告一") || !strings.Contains(out, "正文内容") {
		t.Fatalf("第一份报告被误伤: %q", out)
	}
}

// 单份报告(只有一个一级标题)原样保留,不得误删。
func TestSanitizeReportKeepsSingle(t *testing.T) {
	in := "# 报告\n\n## 方法\n正文。"
	if out := sanitizeReport(in); out != in {
		t.Fatalf("单份报告被改动: %q", out)
	}
}
