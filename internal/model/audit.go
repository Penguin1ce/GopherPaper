package model

import "time"

// AdminAuditLog 记录管理员在后台执行的写操作(POST/PUT/DELETE),用于审计追溯。
type AdminAuditLog struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	AdminID   uint      `gorm:"index" json:"admin_id"`
	AdminName string    `gorm:"size:64" json:"admin_name"`
	Method    string    `gorm:"size:8" json:"method"`
	Path      string    `gorm:"size:255" json:"path"`
	Status    int       `json:"status"`
	IP        string    `gorm:"size:64" json:"ip"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}
