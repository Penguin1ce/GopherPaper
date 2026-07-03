package model

import (
	"time"

	"gorm.io/gorm"
)

// Feedback 是用户提交的反馈/工单,后台处理并可回复。
type Feedback struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	StudentID   string         `gorm:"size:64;index" json:"student_id"`
	Category    string         `gorm:"size:32;index" json:"category"` // bug | feature | other
	Content     string         `gorm:"type:text;not null" json:"content"`
	Status      string         `gorm:"size:16;default:open;index" json:"status"` // open | resolved | closed
	Reply       string         `gorm:"type:text" json:"reply"`
	HandlerName string         `gorm:"size:64" json:"handler_name"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}
