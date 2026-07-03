package model

import (
	"time"

	"gorm.io/gorm"
)

// Announcement 是后台发布的站内公告。
type Announcement struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Title     string         `gorm:"size:255;not null" json:"title"`
	Content   string         `gorm:"type:text" json:"content"`
	Level     string         `gorm:"size:16;default:info" json:"level"` // info | warning | critical
	Published  bool          `gorm:"index" json:"published"`
	AuthorID  uint           `json:"author_id"`
	AuthorName string        `gorm:"size:64" json:"author_name"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
