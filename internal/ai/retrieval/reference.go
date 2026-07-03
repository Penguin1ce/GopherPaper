package retrieval

import (
	"fmt"
	"path/filepath"
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
// BlockType 为 image 时是图块,ImgName 为图片文件名,前端据 DocID+ImgName 拼取图接口渲染缩略图。
type Reference struct {
	ID          string                  `json:"id"`
	Scope       constant.KnowledgeScope `json:"knowledge_scope"`
	StudentID   string                  `json:"student_id,omitempty"`
	DocID       string                  `json:"doc_id,omitempty"`
	SourceFile  string                  `json:"source_file,omitempty"`
	SourceURI   string                  `json:"source_uri,omitempty"`
	PageNo      int64                   `json:"page_no,omitempty"`
	ChunkIndex  int64                   `json:"chunk_index,omitempty"`
	BlockType   string                  `json:"block_type,omitempty"`
	ImgName     string                  `json:"img_name,omitempty"`
	CitationTag string                  `json:"citation_tag,omitempty"`
	Fallback    string                  `json:"fallback_scope,omitempty"`
	Score       float64                 `json:"score,omitempty"`
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
	ref := Reference{
		ID:         doc.ID,
		Scope:      constant.KnowledgeScope(MetaString(doc, constant.MilvusFieldKnowledgeScope)),
		StudentID:  MetaString(doc, constant.MilvusFieldStudentID),
		DocID:      MetaString(doc, constant.MilvusFieldDocID),
		SourceFile: MetaString(doc, constant.MilvusFieldSourceFile),
		SourceURI:  MetaString(doc, constant.MilvusFieldSourceURI),
		PageNo:     metaInt64(doc, constant.MilvusFieldPageNo),
		ChunkIndex: metaInt64(doc, constant.MilvusFieldChunkIndex),
		BlockType:  MetaString(doc, constant.MilvusFieldBlockType),
		Fallback:   MetaString(doc, constant.MetaKeyFallbackScope),
		Score:      doc.Score,
	}
	if uri := MetaString(doc, constant.MilvusFieldImgURI); uri != "" {
		ref.ImgName = filepath.Base(uri)
	}
	ref.CitationTag = FormatCitationTag(ref)
	return ref
}

// FormatDocs 把召回片段拼成带出处的 RAG context,无召回时给模型明确占位。
// chat 与 report 链路共用,保证两处出处格式一致。
func FormatDocs(docs []*Doc) string {
	if len(docs) == 0 {
		return "无相关资料"
	}
	var b strings.Builder
	for i, d := range docs {
		ref := ReferenceFromDocument(d)
		fmt.Fprintf(&b, "[%d] 出处: %s\n", i+1, FormatReference(ref))
		if ref.CitationTag != "" {
			fmt.Fprintf(&b, "citation_tag: %s\n", ref.CitationTag)
		}
		fmt.Fprintf(&b, "%s\n", d.Content)
	}
	return b.String()
}

// FormatReference 把出处按文件、页码、片段、scope 拼成可读串,全空回退到 ID。
func FormatReference(ref Reference) string {
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
	if ref.Fallback == "paper" {
		parts = append(parts, "全文补充")
	}
	if len(parts) == 0 {
		return ref.ID
	}
	return strings.Join(parts, "，")
}

// FormatCitationTag 生成模型可直接复制到正文里的内联出处标签。
// 只有带页码的证据才能形成规范 tag;无页码时返回空串,避免模型编造页码。
func FormatCitationTag(ref Reference) string {
	if ref.PageNo <= 0 {
		return ""
	}
	file := citationFile(ref.SourceFile)
	if file != "" {
		return fmt.Sprintf("[[原文:%s 第 %d 页]]", file, ref.PageNo)
	}
	return fmt.Sprintf("[[原文:第 %d 页]]", ref.PageNo)
}

func citationFile(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = filepath.Base(name)
	replacer := strings.NewReplacer("[", "", "]", "", "\n", " ", "\r", " ")
	return strings.TrimSpace(replacer.Replace(name))
}

func MetaString(doc *Doc, key string) string {
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
