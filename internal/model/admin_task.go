package model

import (
	"time"

	"gorm.io/gorm"
)

// AdminTask 是后台运维任务看板里的一张任务卡片。
// Status 是看板列(todo/doing/review/done),OrderIdx 决定同列内的排序。
type AdminTask struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Title        string         `gorm:"size:255;not null" json:"title"`
	Description  string         `gorm:"type:text" json:"description"`
	Status       string         `gorm:"size:16;index;default:todo" json:"status"`     // todo | doing | review | done
	Priority     string         `gorm:"size:16;index;default:medium" json:"priority"` // low | medium | high | urgent
	Assignee     string         `gorm:"size:64;index" json:"assignee"`
	Labels       JSONStrings    `gorm:"type:text" json:"labels"`
	DueAt        *time.Time     `json:"due_at,omitempty"`
	OrderIdx     int            `gorm:"index" json:"order_idx"`
	CreatorID    uint           `json:"creator_id"`
	CreatorName  string         `gorm:"size:64" json:"creator_name"`
	CommentCount int            `gorm:"-" json:"comment_count"` // 非表列,查询时回填
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (AdminTask) TableName() string { return "admin_tasks" }

// AdminTaskComment 是任务卡片下的一条讨论/进展记录。
type AdminTaskComment struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TaskID     uint      `gorm:"index;not null" json:"task_id"`
	AuthorID   uint      `json:"author_id"`
	AuthorName string    `gorm:"size:64" json:"author_name"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

func (AdminTaskComment) TableName() string { return "admin_task_comments" }

// AdminTaskChecklistItem 是任务下的一条子任务(检查项)。
type AdminTaskChecklistItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TaskID    uint      `gorm:"index;not null" json:"task_id"`
	Content   string    `gorm:"size:512;not null" json:"content"`
	Done      bool      `gorm:"index" json:"done"`
	OrderIdx  int       `gorm:"index" json:"order_idx"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AdminTaskChecklistItem) TableName() string { return "admin_task_checklist_items" }

// AdminTaskActivity 是任务的一条活动记录,构成详情页的时间线。
// Action 取值:created | moved | edited | commented | checklist | assigned | due。
type AdminTaskActivity struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TaskID    uint      `gorm:"index;not null" json:"task_id"`
	ActorID   uint      `json:"actor_id"`
	ActorName string    `gorm:"size:64" json:"actor_name"`
	Action    string    `gorm:"size:24;not null" json:"action"`
	Detail    string    `gorm:"size:512" json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

func (AdminTaskActivity) TableName() string { return "admin_task_activities" }
