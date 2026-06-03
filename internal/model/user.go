// Package model 定义 GORM 业务数据模型。
package model

import (
	"time"

	"gorm.io/gorm"
)

// User 是学生用户。StudentID 学号既是登录账号，
// 也是 RAG 私有库 partition 与租户上下文的隔离键。
type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	StudentID    string         `gorm:"size:64;uniqueIndex;not null" json:"student_id"`
	Name         string         `gorm:"size:64" json:"name"`
	Email        string         `gorm:"size:128;index" json:"email"`
	ClassID      string         `gorm:"size:64;index" json:"class_id"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string { return "users" }
