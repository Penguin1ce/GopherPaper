package sessiontitle

import (
	"strings"
	"testing"

	"GopherPaper/pkg/constant"
)

func TestCleanSessionTitle(t *testing.T) {
	got := Clean("标题： 「这篇论文的方法流程是什么？」\n解释")
	if got != "方法流程是什么" {
		t.Fatalf("Clean() = %q", got)
	}
}

func TestCleanSessionTitleFromJSON(t *testing.T) {
	got := Clean(`{"title":"消融实验结论"}`)
	if got != "消融实验结论" {
		t.Fatalf("Clean(JSON) = %q", got)
	}
}

func TestCleanSessionTitleKeepsTechnicalQuestionAnswering(t *testing.T) {
	got := Clean("请问这篇论文的视觉问答模型怎么训练？")
	if got != "视觉问答模型怎么训练" {
		t.Fatalf("Clean(VQA) = %q", got)
	}
}

func TestFallbackTruncatesLongQuestion(t *testing.T) {
	got := Fallback(strings.Repeat("长", constant.SessionDisplayTitleMaxRunes+8))
	if len([]rune(got)) != constant.SessionDisplayTitleMaxRunes {
		t.Fatalf("fallback length = %d, want %d", len([]rune(got)), constant.SessionDisplayTitleMaxRunes)
	}
}
