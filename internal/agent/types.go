// Package agent 定义各下游 agent 共享的输入输出契约。
package agent

import "GopherPaper/pkg/constant"

// Intent 是意图识别小模型的结构化输出。
type Intent struct {
	Type  constant.IntentType `json:"type"`
	Slots map[string]string   `json:"slots"` // 抽取出的参数，如知识点、数量、难度
}

// Request 是一次助教请求的原始输入。学生身份经 context 透传，不放这里。
type Request struct {
	Query string `json:"query"`
}

// AgentInput 是 Branch 之后分发给具体 agent 的统一输入。
type AgentInput struct {
	Query  string
	Intent Intent
}

// Reply 是所有 agent 的统一输出。
type Reply struct {
	Intent  constant.IntentType `json:"intent"`
	Content string              `json:"content"`
	Meta    map[string]any      `json:"meta,omitempty"` // agent 特有的结构化数据
}

// ParsedDoc 是 PDF 解析后的结构化中间产物，由 parser 产出，供抽取与分块共用。
// 不与具体解析器耦合，MinerU 的细节在 parser 包内消化。
type ParsedDoc struct {
	Sections   []Section   `json:"sections"`   // 章节标题树，层级即 Level
	Paragraphs []Paragraph `json:"paragraphs"` // 正文段落，带页码与所属章节路径
	Figures    []Figure    `json:"figures"`    // 图表说明
	References []string    `json:"references"` // 参考文献，整条保留
	PageCount  int         `json:"page_count"`
}

// Section 是一个章节标题节点。
type Section struct {
	Level    int    `json:"level"` // 标题层级，1 为顶级
	Title    string `json:"title"`
	PageNo   int    `json:"page_no"`
	OrderIdx int    `json:"order_idx"` // 文档内顺序
}

// Paragraph 是一段正文，PageNo 为所在页，SectionPath 为所属章节标题链。
type Paragraph struct {
	Text        string `json:"text"`
	PageNo      int    `json:"page_no"`
	SectionPath string `json:"section_path"`
}

// Figure 是一条图表说明。
type Figure struct {
	Caption string `json:"caption"`
	PageNo  int    `json:"page_no"`
}

// PaperStructured 是论文结构化抽取结果，由 extract_pipeline 产出。
type PaperStructured struct {
	Title             string   `json:"title"`
	Authors           []string `json:"authors"`
	Affiliations      []string `json:"affiliations"`
	Abstract          string   `json:"abstract"`
	Keywords          []string `json:"keywords"`
	ResearchQuestions []string `json:"research_questions"`
	Methods           string   `json:"methods"`
	Experiments       string   `json:"experiments"`
	Results           string   `json:"results"`
	Innovations       []string `json:"innovations"`
	Limitations       []string `json:"limitations"`
	FutureWork        []string `json:"future_work"`
}

// ReportInput 是研读报告 agent 的输入：围绕某篇论文按类型生成。
type ReportInput struct {
	PaperID    string              `json:"paper_id"`
	OwnerID    string              `json:"owner_id"`
	ReportType constant.ReportType `json:"report_type"`
	Query      string              `json:"query"` // 检索用查询，默认用论文标题或类型关键词
}
