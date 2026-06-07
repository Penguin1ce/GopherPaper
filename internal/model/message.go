package model

import (
	"time"

	"GopherPaper/pkg/constant"
)

// 消息角色，与 trpc model 的 role 取值保持一致。
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// Message 是会话里的一条对话内容。会话历史已迁到 trpc Session（Redis 事件），
// 不再落 MySQL，故无 gorm 标签：本结构仅作 HTTP 层的传输载体。
// ID 取自底层 Session 事件的 ID（字符串）；新生成、尚未落 Session 的助教消息 ID 为空。
// Intent 仅助教消息有值，按 created_at 升序还原上下文。
type Message struct {
	ID        string              `json:"id"`
	SessionID string              `json:"session_id"`
	Role      string              `json:"role"`
	Content   string              `json:"content"`
	Intent    constant.IntentType `json:"intent,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
}
