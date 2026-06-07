package model

import "gorm.io/gorm"

// AutoMigrate 建表，开发期使用，生产建议改用迁移工具。
func AutoMigrate(db *gorm.DB) error {
	// Message 已迁到 trpc Session（Redis），不再建 messages 表。
	return db.AutoMigrate(
		&User{}, &Session{},
		&Paper{}, &PaperMeta{}, &PaperSection{}, &Tag{}, &PaperTag{},
	)
}
