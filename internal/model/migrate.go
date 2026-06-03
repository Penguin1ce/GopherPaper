package model

import "gorm.io/gorm"

// AutoMigrate 建表，开发期使用，生产建议改用迁移工具。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{}, &Session{}, &Message{})
}
