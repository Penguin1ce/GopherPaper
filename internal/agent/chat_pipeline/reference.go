package chat_pipeline

import (
	"fmt"
	"strings"

	"GopherPaper/pkg/constant"
)

// Doc 是一条召回片段的数据容器,承载检索结果的 id/正文/metadata/score(仅数据载体,非编排)。
type Doc struct {
	ID       string
	Content  string
	MetaData map[string]any
	Score    float64
}

// Reference 是一条召回片段的出处，回传前端渲染引用。
type Reference struct {
	ID         string                  `json:"id"`
	Scope      constant.KnowledgeScope `json:"scope"`
	StudentID  string                  `json:"student_id,omitempty"`
	DocID      string                  `json:"doc_id,omitempty"`
	SourceFile string                  `json:"source_file,omitempty"`
	SourceURI  string                  `json:"source_uri,omitempty"`
	PageNo     int64                   `json:"page_no,omitempty"`
	ChunkIndex int64                   `json:"chunk_index,omitempty"`
	Score      float64                 `json:"score,omitempty"`
}

func References(docs []*Doc) []Reference {
	refs := make([]Reference, 0, len(docs))
	for _, doc := range docs {
		refs = append(refs, ReferenceFromDocument(doc))
	}
	return refs
}

func ReferenceFromDocument(doc *Doc) Reference {
	if doc == nil {
		return Reference{}
	}
	return Reference{
		ID:         doc.ID,
		Scope:      constant.KnowledgeScope(metaString(doc, constant.MilvusFieldKnowledgeScope)),
		StudentID:  metaString(doc, constant.MilvusFieldStudentID),
		DocID:      metaString(doc, constant.MilvusFieldDocID),
		SourceFile: metaString(doc, constant.MilvusFieldSourceFile),
		SourceURI:  metaString(doc, constant.MilvusFieldSourceURI),
		PageNo:     metaInt64(doc, constant.MilvusFieldPageNo),
		ChunkIndex: metaInt64(doc, constant.MilvusFieldChunkIndex),
		Score:      doc.Score,
	}
}

// formatDocs 把召回片段拼成带出处的 RAG context,无召回时给模型明确占位。
func formatDocs(docs []*Doc) string {
	if len(docs) == 0 {
		return "无相关资料"
	}
	var b strings.Builder
	for i, d := range docs {
		ref := ReferenceFromDocument(d)
		fmt.Fprintf(&b, "[%d] 出处: %s\n%s\n", i+1, formatReference(ref), d.Content)
	}
	return b.String()
}

// formatReference 把出处按文件、页码、片段、scope 拼成可读串,全空回退到 ID。
func formatReference(ref Reference) string {
	parts := []string{}
	if ref.SourceFile != "" {
		parts = append(parts, ref.SourceFile)
	}
	if ref.SourceURI != "" {
		parts = append(parts, ref.SourceURI)
	}
	if ref.PageNo > 0 {
		parts = append(parts, fmt.Sprintf("第 %d 页", ref.PageNo))
	}
	if ref.ChunkIndex > 0 {
		parts = append(parts, fmt.Sprintf("片段 %d", ref.ChunkIndex))
	}
	if ref.Scope != "" {
		parts = append(parts, string(ref.Scope))
	}
	if len(parts) == 0 {
		return ref.ID
	}
	return strings.Join(parts, "，")
}

func metaString(doc *Doc, key string) string {
	if doc == nil || doc.MetaData == nil {
		return ""
	}
	v, ok := doc.MetaData[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func metaInt64(doc *Doc, key string) int64 {
	if doc == nil || doc.MetaData == nil {
		return 0
	}
	switch v := doc.MetaData[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}
