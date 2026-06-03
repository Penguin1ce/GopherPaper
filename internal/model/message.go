package model

import (
	"time"

	"GopherCPP/pkg/constant"
)

// 消息角色，与 eino schema 保持一致。
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// Message 是会话里的一条对话内容，追加写、不可变，
// 按 created_at 升序还原上下文。Intent 仅用户消息有值。
type Message struct {
	ID        uint64              `gorm:"primaryKey" json:"id"`
	SessionID string              `gorm:"size:36;not null;index:idx_session_created" json:"session_id"`
	Role      string              `gorm:"size:16;not null" json:"role"`
	Content   string              `gorm:"type:text;not null" json:"content"`
	Intent    constant.IntentType `gorm:"size:16" json:"intent,omitempty"`
	CreatedAt time.Time           `gorm:"not null;index:idx_session_created" json:"created_at"`
}

func (Message) TableName() string { return "messages" }
