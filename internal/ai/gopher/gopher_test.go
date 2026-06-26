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
