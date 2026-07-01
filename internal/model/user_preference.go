package model

import "time"

// UserPreference 保存用户对 AI 回答方式的受控偏好。
type UserPreference struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	StudentID         string    `gorm:"size:64;uniqueIndex;not null" json:"student_id"`
	Nickname          string    `gorm:"size:64" json:"nickname"`
	AnswerStyle       string    `gorm:"size:32;not null" json:"answer_style"`
	OutputFormat      string    `gorm:"size:32;not null" json:"output_format"`
	Language          string    `gorm:"size:32;not null" json:"language"`
	CustomInstruction string    `gorm:"size:1000" json:"custom_instruction"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (UserPreference) TableName() string { return "user_preferences" }
