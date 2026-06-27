package model

import "gorm.io/gorm"

// AutoMigrate 建表，仅供开发期使用。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{}, &Session{},
		&ServiceCallLog{},
		&Paper{}, &PaperMeta{}, &PaperSection{}, &PaperReport{}, &Tag{}, &PaperTag{},
	)
}
