package model

import "time"

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
