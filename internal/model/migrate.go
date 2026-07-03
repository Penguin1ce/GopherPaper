package model

import "gorm.io/gorm"

// AutoMigrate 建表，仅供开发期使用。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{}, &UserPreference{}, &Session{}, &Topic{},
		&Admin{}, &ServiceCallLog{}, &SystemModelConfig{}, &AdminAuditLog{},
		&SystemSetting{}, &Announcement{}, &Feedback{},
		&Paper{}, &PaperMeta{}, &PaperSection{}, &PaperReport{}, &PaperFlowCache{}, &PaperCompareReport{}, &PaperAnnotation{}, &MindMap{}, &Tag{}, &PaperTag{},
		&PaperSemanticRelation{},
	)
}
