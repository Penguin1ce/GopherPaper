package model

import "time"

// SystemSetting 是后台可配置的键值项,按 group 分组展示。
type SystemSetting struct {
	Key       string    `gorm:"size:64;primaryKey" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	Group     string    `gorm:"size:32;index" json:"group"`
	Label     string    `gorm:"size:128" json:"label"`
	Type      string    `gorm:"size:16" json:"type"` // string | bool | number
	UpdatedAt time.Time `json:"updated_at"`
}
