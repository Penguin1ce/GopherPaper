package chat_pipeline

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"GopherCPP/pkg/constant"
)

// TestVisibleFilter_EmptyStudentOnlyPublic 无学生身份时只能看到公共库。
func TestVisibleFilter_EmptyStudentOnlyPublic(t *testing.T) {
	got := visibleFilter("")
	want := "knowledge_scope == 'public'"
	if got != want {
		t.Fatalf("空 studentID 应只过滤公共库\n want %q\n got  %q", want, got)
	}
}

// TestVisibleFilter_WithStudent 有学生身份时是公共库或本人私有库的并集。
func TestVisibleFilter_WithStudent(t *testing.T) {
	got := visibleFilter("s_42")
	for _, sub := range []string{
		"knowledge_scope == 'public'",
		"knowledge_scope == 'private'",
		"student_id == 's_42'",
		" or ",
	} {
		if !strings.Contains(got, sub) {
			t.Fatalf("过滤表达式缺少 %q\n got %q", sub, got)
		}
	}
	// 不应出现别的学生
	if strings.Contains(got, "s_43") {
		t.Fatalf("过滤表达式泄漏了其他学生: %q", got)
	}
}

// TestVisibleFilter_EscapesQuote 学生 ID 含单引号时必须转义，防止过滤表达式注入。
func TestVisibleFilter_EscapesQuote(t *testing.T) {
	got := visibleFilter("a'b")
	if !strings.Contains(got, `student_id == 'a\'b'`) {
		t.Fatalf("单引号未正确转义: %q", got)
	}
}

// TestReferenceFromDocument 从检索文档的 metadata 还原出处。
func TestReferenceFromDocument(t *testing.T) {
	doc := &schema.Document{
		ID: "chunk-1",
		MetaData: map[string]any{
			constant.MilvusFieldKnowledgeScope: "private",
			constant.MilvusFieldStudentID:      "s_1",
			constant.MilvusFieldDocID:          "doc-9",
			constant.MilvusFieldSourceFile:     "ch01.pdf",
			constant.MilvusFieldSourceURI:      "oss://bucket/ch01.pdf",
			constant.MilvusFieldPageNo:         int64(3),
			constant.MilvusFieldChunkIndex:     int64(2),
		},
	}
	doc.WithScore(0.87)

	ref := ReferenceFromDocument(doc)
	if ref.ID != "chunk-1" || ref.Scope != constant.KnowledgeScopePrivate {
		t.Fatalf("ID/Scope 提取错误: %+v", ref)
	}
	if ref.StudentID != "s_1" || ref.DocID != "doc-9" {
		t.Fatalf("StudentID/DocID 提取错误: %+v", ref)
	}
	if ref.SourceFile != "ch01.pdf" || ref.SourceURI != "oss://bucket/ch01.pdf" {
		t.Fatalf("出处文件提取错误: %+v", ref)
	}
	if ref.PageNo != 3 || ref.ChunkIndex != 2 {
		t.Fatalf("页码/片段号提取错误: %+v", ref)
	}
	if ref.Score != 0.87 {
		t.Fatalf("Score 提取错误: %+v", ref)
	}
}

// TestFormatReference 出处按文件、页码、片段、scope 拼成可读串。
func TestFormatReference(t *testing.T) {
	ref := Reference{
		SourceFile: "ch01.pdf",
		PageNo:     3,
		Scope:      constant.KnowledgeScopePublic,
	}
	got := formatReference(ref)
	want := "ch01.pdf，第 3 页，public"
	if got != want {
		t.Fatalf("出处格式不符\n want %q\n got  %q", want, got)
	}
}

// TestFormatReference_FallbackToID 无任何出处字段时回退到 ID。
func TestFormatReference_FallbackToID(t *testing.T) {
	if got := formatReference(Reference{ID: "only-id"}); got != "only-id" {
		t.Fatalf("应回退到 ID, got %q", got)
	}
}

// TestFormatDocs_Empty 无召回时给模型一个明确的占位。
func TestFormatDocs_Empty(t *testing.T) {
	if got := formatDocs(nil); got != "无相关资料" {
		t.Fatalf("空召回占位错误: %q", got)
	}
}
