// Package core 定义 ai 子系统共享的输入输出契约。
package core

import "GopherPaper/pkg/constant"

// Reply 是所有 ai 链路的统一输出。
type Reply struct {
	Intent  constant.IntentType `json:"intent"`
	Content string              `json:"content"`
	Meta    map[string]any      `json:"meta,omitempty"` // 链路特有的结构化数据
}

// ParsedDoc 是 PDF 解析后的结构化中间产物，由 parser 产出，供抽取与分块共用。
// 不与具体解析器耦合，MinerU 的细节在 parser 包内消化。
type ParsedDoc struct {
	Sections   []Section   `json:"sections"`    // 章节标题树，层级即 Level
	Paragraphs []Paragraph `json:"paragraphs"`  // 正文段落，带页码与所属章节路径
	Figures    []Figure    `json:"figures"`     // 插图说明
	Tables     []Table     `json:"tables"`      // 表格，MinerU table_body 转 Markdown/行列网格入库
	CodeBlocks []CodeBlock `json:"code_blocks"` // 算法、伪代码与提示词块，独立入库便于方法检索
	References []string    `json:"references"`  // 参考文献，整条保留
	PageCount  int         `json:"page_count"`
	Artifact   []byte      `json:"-"` // MinerU 原始产物 zip 字节，仅 worker 内存传递供归档落盘，不序列化
}

// Section 是一个章节标题节点。
type Section struct {
	Level    int      `json:"level"` // 标题层级，1 为顶级
	Title    string   `json:"title"`
	PageNo   int      `json:"page_no"`
	OrderIdx int      `json:"order_idx"` // 文档内顺序
	X1       *float64 `json:"x1,omitempty"`
	Y1       *float64 `json:"y1,omitempty"`
	X2       *float64 `json:"x2,omitempty"`
	Y2       *float64 `json:"y2,omitempty"`
}

// Paragraph 是一段正文，PageNo 为所在页，SectionPath 为所属章节标题链。
type Paragraph struct {
	Text        string `json:"text"`
	PageNo      int    `json:"page_no"`
	SectionPath string `json:"section_path"`
}

// Figure 是一条图表说明。
// ImgPath 为产物 zip 内的相对路径,ImgData 为解码出的图片字节(仅 worker 内存传递,不序列化),
// ImgURI 在 worker 把图片落盘后回填为本地路径,供建图块与带图问答用。
type Figure struct {
	Caption     string `json:"caption"`
	Text        string `json:"text,omitempty"` // MinerU/VLM 后端给出的图表正文描述,与 caption/Desc 一起入库
	PageNo      int    `json:"page_no"`
	SectionPath string `json:"section_path,omitempty"` // 所属章节标题链,前缀进图块文本提升召回
	ImgPath     string `json:"img_path,omitempty"`
	ImgData     []byte `json:"-"`
	ImgURI      string `json:"img_uri,omitempty"`
	Desc        string `json:"desc,omitempty"` // vlm 解析期生成的图片内容描述,与 caption 一起入库提升召回
}

// Table 是一张表格。MinerU 的 table_body 已转成 Markdown 与 Rows 网格,
// Caption 为表题与脚注合并。表格走文本入库;若 MinerU 同时给了表格截图,
// parser 会额外生成 Figure 供原图展示与 VLM 描述。
type Table struct {
	Caption     string     `json:"caption"`
	Markdown    string     `json:"markdown"`
	PageNo      int        `json:"page_no"`
	SectionPath string     `json:"section_path,omitempty"` // 所属章节标题链,前缀进表块文本提升召回
	ImgPath     string     `json:"img_path,omitempty"`
	ImgData     []byte     `json:"-"`
	ImgURI      string     `json:"img_uri,omitempty"`
	Rows        [][]string `json:"rows,omitempty"`
}

// CodeBlock 是论文中的算法、伪代码或 prompt 代码块。
type CodeBlock struct {
	Caption     string `json:"caption,omitempty"`
	Body        string `json:"body"`
	Language    string `json:"language,omitempty"`
	PageNo      int    `json:"page_no"`
	SectionPath string `json:"section_path"`
}

// PaperStructured 是论文结构化抽取结果，由 extract 产出。
type PaperStructured struct {
	Title             string   `json:"title"`
	Authors           []string `json:"authors"`
	Affiliations      []string `json:"affiliations"`
	PublishYear       int      `json:"publish_year"`
	Venue             string   `json:"venue"`
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

// PaperCompareInput 是多论文对比生成的结构化输入。
type PaperCompareInput struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	FileName          string   `json:"file_name"`
	Authors           []string `json:"authors"`
	PublishYear       int      `json:"publish_year"`
	Venue             string   `json:"venue"`
	Keywords          []string `json:"keywords"`
	ResearchQuestions []string `json:"research_questions"`
	Methods           string   `json:"methods"`
	Experiments       string   `json:"experiments"`
	Results           string   `json:"results"`
}

// ReportInput 是研读报告输入：围绕某篇论文按类型生成。论文 owner 从 ctx 的 tenant 取。
type ReportInput struct {
	PaperID    string              `json:"paper_id"`
	ReportType constant.ReportType `json:"report_type"`
	Query      string              `json:"query"` // 检索用查询，默认用论文标题或类型关键词
}
