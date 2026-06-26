package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"GopherPaper/pkg/constant"
)

// JSONStrings 是落库为 JSON 文本的字符串数组，用于作者/关键词等多值字段。
type JSONStrings []string

func (s JSONStrings) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (s *JSONStrings) Scan(src any) error {
	if src == nil {
		*s = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("model: JSONStrings 不支持的类型 %T", src)
	}
	if len(b) == 0 {
		*s = nil
		return nil
	}
	return json.Unmarshal(b, s)
}

// JSONMap 是落库为 JSON 文本的键值表，用于报告附带的结构化元数据。
type JSONMap map[string]any

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	return string(b), err
}

func (m *JSONMap) Scan(src any) error {
	if src == nil {
		*m = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("model: JSONMap 不支持的类型 %T", src)
	}
	if len(b) == 0 {
		*m = nil
		return nil
	}
	return json.Unmarshal(b, m)
}

// Paper 是一篇上传的论文，归属某个用户,主键 UUID 便于对外暴露。
// Status 是从上传到就绪的解析状态机。
type Paper struct {
	ID         string               `gorm:"size:36;primaryKey" json:"id"`
	OwnerID    string               `gorm:"size:64;index;not null" json:"owner_id"`
	Title      string               `gorm:"size:512" json:"title"`
	FileName   string               `gorm:"size:512;not null" json:"file_name"`
	FileURI    string               `gorm:"size:1024;not null" json:"file_uri"`
	Size       int64                `json:"size"`
	Status     constant.PaperStatus `gorm:"size:16;not null;index" json:"status"`
	FailReason string               `gorm:"size:512" json:"fail_reason,omitempty"`
	PageCount  int                  `json:"page_count"`
	Category   string               `gorm:"size:64;index" json:"category,omitempty"`
	Progress   int                  `json:"progress"`                    // 阅读进度 0-100
	Keywords   JSONStrings          `gorm:"-" json:"keywords,omitempty"` // 非表列 从 paper_meta 回填供前端筛选
	CreatedAt  time.Time            `json:"created_at"`
	UpdatedAt  time.Time            `json:"updated_at"`
	DeletedAt  gorm.DeletedAt       `gorm:"index" json:"-"`
}

func (Paper) TableName() string { return "papers" }

// BeforeCreate 入库前自动生成 UUID 主键并设默认状态。
func (p *Paper) BeforeCreate(*gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.Status == "" {
		p.Status = constant.PaperUploaded
	}
	return nil
}

// PaperMeta 是论文结构化抽取结果，与 Paper 一对一。
type PaperMeta struct {
	PaperID           string      `gorm:"size:36;primaryKey" json:"paper_id"`
	Authors           JSONStrings `gorm:"type:text" json:"authors"`
	Affiliations      JSONStrings `gorm:"type:text" json:"affiliations"`
	PublishYear       int         `json:"publish_year"`
	Venue             string      `gorm:"size:256" json:"venue"`
	Abstract          string      `gorm:"type:text" json:"abstract"`
	Keywords          JSONStrings `gorm:"type:text" json:"keywords"`
	ResearchQuestions JSONStrings `gorm:"type:text" json:"research_questions"`
	Methods           string      `gorm:"type:text" json:"methods"`
	Experiments       string      `gorm:"type:text" json:"experiments"`
	Results           string      `gorm:"type:text" json:"results"`
	Innovations       JSONStrings `gorm:"type:text" json:"innovations"`
	Limitations       JSONStrings `gorm:"type:text" json:"limitations"`
	FutureWork        JSONStrings `gorm:"type:text" json:"future_work"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

func (PaperMeta) TableName() string { return "paper_metas" }

// PaperSection 是论文章节标题节点，保留段落层级供前端展示。
type PaperSection struct {
	ID       uint64 `gorm:"primaryKey" json:"id"`
	PaperID  string `gorm:"size:36;not null;index" json:"paper_id"`
	Level    int    `json:"level"`
	Title    string `gorm:"size:512" json:"title"`
	PageNo   int    `json:"page_no"`
	OrderIdx int    `gorm:"index" json:"order_idx"`
}

func (PaperSection) TableName() string { return "paper_sections" }

// PaperReport 是某篇论文某类研读报告的持久化结果，按 (paper_id, report_type) 唯一。
// 同一类型报告生成一次后落库，再次点击同一按钮命中缓存直接复用,不重复调模型。
type PaperReport struct {
	ID         uint64              `gorm:"primaryKey" json:"-"`
	PaperID    string              `gorm:"size:36;not null;uniqueIndex:idx_paper_report_type" json:"paper_id"`
	ReportType constant.ReportType `gorm:"size:16;not null;uniqueIndex:idx_paper_report_type" json:"report_type"`
	Content    string              `gorm:"type:longtext" json:"content"`
	Meta       JSONMap             `gorm:"type:text" json:"meta,omitempty"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

func (PaperReport) TableName() string { return "paper_reports" }

// Tag 是用户自定义的论文标签。
type Tag struct {
	ID      uint64 `gorm:"primaryKey" json:"id"`
	OwnerID string `gorm:"size:64;not null;index:idx_owner_name" json:"owner_id"`
	Name    string `gorm:"size:64;not null;index:idx_owner_name" json:"name"`
}

func (Tag) TableName() string { return "tags" }

// PaperTag 是论文与标签的多对多关联。
type PaperTag struct {
	PaperID string `gorm:"size:36;primaryKey" json:"paper_id"`
	TagID   uint64 `gorm:"primaryKey" json:"tag_id"`
}

func (PaperTag) TableName() string { return "paper_tags" }
