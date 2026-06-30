package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MemoryItem 是用户可控的长期记忆卡片,第一版只落 MySQL。
type MemoryItem struct {
	ID            string         `gorm:"size:36;primaryKey" json:"id"`
	StudentID     string         `gorm:"size:64;not null;index:idx_memory_owner_type" json:"student_id"`
	Type          string         `gorm:"size:32;not null;index:idx_memory_owner_type" json:"type"`
	Title         string         `gorm:"size:160;not null" json:"title"`
	Content       string         `gorm:"type:longtext;not null" json:"content"`
	Tags          JSONStrings    `gorm:"type:text" json:"tags"`
	SourcePaperID string         `gorm:"size:36;index" json:"source_paper_id,omitempty"`
	Pinned        bool           `gorm:"index" json:"pinned"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (MemoryItem) TableName() string { return "memory_items" }

func (m *MemoryItem) BeforeCreate(*gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	if m.Type == "" {
		m.Type = "论文笔记"
	}
	return nil
}
