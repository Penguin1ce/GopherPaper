package dto

import (
	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
)

// PaperStatusResponse 是论文解析状态查询响应。
type PaperStatusResponse struct {
	ID            string               `json:"id"`
	Status        constant.PaperStatus `json:"status"`
	FailReason    string               `json:"fail_reason"`
	ParseProgress int                  `json:"parse_progress"`
	ParsedPages   int                  `json:"parsed_pages"`
	TotalPages    int                  `json:"total_pages"`
}

// PaperDetailResponse 是论文详情页响应。
type PaperDetailResponse struct {
	Paper    *model.Paper         `json:"paper"`
	Meta     *model.PaperMeta     `json:"meta"`
	Sections []model.PaperSection `json:"sections"`
}

// ReportProgressStep 是报告生成过程中的一个执行计划片段。
type ReportProgressStep struct {
	Phase string `json:"phase"`
	Text  string `json:"text"`
}

// ReportRun 是某类报告当前生成态的可恢复快照。
type ReportRun struct {
	Type   constant.ReportType  `json:"type"`
	Steps  []ReportProgressStep `json:"steps"`
	Live   bool                 `json:"live"`
	Failed bool                 `json:"failed"`
}

// ReadyReportsResponse 是某篇论文已生成和生成中的研读报告状态。
type ReadyReportsResponse struct {
	Ready   []constant.ReportType `json:"ready"`
	Running []ReportRun           `json:"running"`
}

// TranslateResponse 是选段翻译响应。
type TranslateResponse struct {
	Translation string `json:"translation"`
}

// GraphStats 是某用户知识图谱的总览统计。
type GraphStats struct {
	Papers    int `json:"papers"`
	Authors   int `json:"authors"`
	Keywords  int `json:"keywords"`
	Citations int `json:"citations"`
	MinYear   int `json:"min_year"`
	MaxYear   int `json:"max_year"`
}

// YearCount 是某一年的论文数量。
type YearCount struct {
	Year  int `json:"year"`
	Count int `json:"count"`
}

// KeywordYearCount 是某关键词在某年的出现次数。
type KeywordYearCount struct {
	Keyword string `json:"keyword"`
	Year    int    `json:"year"`
	Count   int    `json:"count"`
}

// GraphTrendsResponse 是图谱研究趋势响应。
type GraphTrendsResponse struct {
	ByYear       []YearCount        `json:"by_year"`
	KeywordTrend []KeywordYearCount `json:"keyword_trend"`
}

// NameCount 是名称与计数,用于热门关键词。
type NameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// RelatedPaper 是与某篇论文相关的论文。
type RelatedPaper struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Year  int      `json:"year"`
	Score int      `json:"score"`
	Vias  []string `json:"vias"`
}
