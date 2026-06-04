package chat_pipeline

import (
	"fmt"

	"github.com/cloudwego/eino/schema"

	"GopherPaper/pkg/constant"
)

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

func References(docs []*schema.Document) []Reference {
	refs := make([]Reference, 0, len(docs))
	for _, doc := range docs {
		refs = append(refs, ReferenceFromDocument(doc))
	}
	return refs
}

func ReferenceFromDocument(doc *schema.Document) Reference {
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
		Score:      doc.Score(),
	}
}

func metaString(doc *schema.Document, key string) string {
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

func metaInt64(doc *schema.Document, key string) int64 {
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
