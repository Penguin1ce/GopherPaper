package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Session 是一段多轮对话，归属某个学生，主键为 UUID 便于对外暴露。
type Session struct {
	ID        string         `gorm:"size:36;primaryKey" json:"id"`
	StudentID string         `gorm:"size:64;index;not null" json:"student_id"`
	Title     string         `gorm:"size:128" json:"title"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Session) TableName() string { return "sessions" }

// BeforeCreate 在入库前自动生成 UUID 主键。
func (s *Session) BeforeCreate(*gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	return nil
}
