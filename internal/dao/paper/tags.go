package paper

import (
	"context"
	"fmt"

	"gorm.io/gorm/clause"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/model"
)

// EnsureTag 取或建某用户名下的标签，返回标签 ID。
func EnsureTag(ctx context.Context, ownerID, name string) (uint64, error) {
	tag := model.Tag{OwnerID: ownerID, Name: name}
	err := dao.DB.WithContext(ctx).
		Where("owner_id = ? and name = ?", ownerID, name).
		FirstOrCreate(&tag).Error
	if err != nil {
		return 0, fmt.Errorf("dao/paper: 取或建标签失败: %w", err)
	}
	return tag.ID, nil
}

// ListTags 列出某用户的全部标签。
func ListTags(ctx context.Context, ownerID string) ([]model.Tag, error) {
	var tags []model.Tag
	err := dao.DB.WithContext(ctx).
		Where("owner_id = ?", ownerID).
		Order("name asc").
		Find(&tags).Error
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 查询标签失败: %w", err)
	}
	return tags, nil
}

// AttachTag 给论文打标签，重复打标幂等。
func AttachTag(ctx context.Context, paperID string, tagID uint64) error {
	pt := model.PaperTag{PaperID: paperID, TagID: tagID}
	err := dao.DB.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&pt).Error
	if err != nil {
		return fmt.Errorf("dao/paper: 打标签失败: %w", err)
	}
	return nil
}

// DetachTag 移除论文上的某个标签。
func DetachTag(ctx context.Context, paperID string, tagID uint64) error {
	err := dao.DB.WithContext(ctx).
		Where("paper_id = ? and tag_id = ?", paperID, tagID).
		Delete(&model.PaperTag{}).Error
	if err != nil {
		return fmt.Errorf("dao/paper: 移除标签失败: %w", err)
	}
	return nil
}

// TagsOfPaper 列出某论文的标签 ID。
func TagsOfPaper(ctx context.Context, paperID string) ([]uint64, error) {
	var ids []uint64
	err := dao.DB.WithContext(ctx).
		Model(&model.PaperTag{}).
		Where("paper_id = ?", paperID).
		Pluck("tag_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("dao/paper: 查询论文标签失败: %w", err)
	}
	return ids, nil
}
