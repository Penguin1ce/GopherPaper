package model

import (
	"time"

	"gorm.io/gorm"
)

type Admin struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Username     string         `gorm:"size:64;uniqueIndex;not null" json:"username"`
	Email        string         `gorm:"size:128;uniqueIndex;not null" json:"email"`
	Name         string         `gorm:"size:64" json:"name"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	Status       string         `gorm:"size:16;not null;default:active;index" json:"status"`
	LastLoginAt  *time.Time     `json:"last_login_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Admin) TableName() string { return "admins" }

type ServiceCallLog struct {
	ID           uint64    `gorm:"primaryKey" json:"id"`
	ServiceType  string    `gorm:"size:32;not null;index" json:"service_type"`
	ActorID      string    `gorm:"size:64;index" json:"actor_id,omitempty"`
	PaperID      string    `gorm:"size:36;index" json:"paper_id,omitempty"`
	SessionID    string    `gorm:"size:36;index" json:"session_id,omitempty"`
	Success      bool      `gorm:"index" json:"success"`
	DurationMS   int64     `json:"duration_ms"`
	ErrorMessage string    `gorm:"size:1024" json:"error_message,omitempty"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}

func (ServiceCallLog) TableName() string { return "service_call_logs" }
