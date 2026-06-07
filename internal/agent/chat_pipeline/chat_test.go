package chat_pipeline

import (
	"encoding/json"
	"testing"

	"GopherPaper/pkg/constant"
)

// TestReferenceFromDocument 从检索文档的 metadata 还原出处。
func TestReferenceFromDocument(t *testing.T) {
	doc := &Doc{
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
		Score: 0.87,
	}

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

// TestReferenceJSONContract 校验前后端引用出处字段契约。
func TestReferenceJSONContract(t *testing.T) {
	ref := Reference{
		ID:    "chunk-1",
		Scope: constant.KnowledgeScopePublic,
	}
	b, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("Reference 序列化失败: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatalf("Reference 反序列化失败: %v", err)
	}
	if body["knowledge_scope"] != string(constant.KnowledgeScopePublic) {
		t.Fatalf("knowledge_scope 字段错误: %s", string(b))
	}
	if _, ok := body["scope"]; ok {
		t.Fatalf("不应输出旧字段 scope: %s", string(b))
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
