package model

import "gorm.io/gorm"

// AutoMigrate 建表，仅供开发期使用。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{}, &Session{}, &Topic{},
		&Admin{}, &ServiceCallLog{},
		&Paper{}, &PaperMeta{}, &PaperSection{}, &PaperReport{}, &PaperAnnotation{}, &MindMap{}, &Tag{}, &PaperTag{},
		&PaperSemanticRelation{},
	)
}
