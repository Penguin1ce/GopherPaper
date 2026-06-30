// Package memory 管理用户可控的个人长期记忆卡片。
package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/errs"
)

const (
	maxTitleRunes   = 160
	maxContentRunes = 20000
	maxTypeRunes    = 32
	maxTagRunes     = 32
	maxTags         = 12
)

func List(ctx context.Context, studentID string, req dto.MemorySearchRequest) ([]model.MemoryItem, error) {
	var items []model.MemoryItem
	db := dao.DB.WithContext(ctx).Where("student_id = ?", studentID)
	if t := strings.TrimSpace(req.Type); t != "" {
		db = db.Where("type = ?", t)
	}
	if tag := strings.TrimSpace(req.Tag); tag != "" {
		db = db.Where("tags LIKE ?", "%"+escapeLike(tag)+"%")
	}
	if q := strings.TrimSpace(req.Q); q != "" {
		like := "%" + escapeLike(q) + "%"
		db = db.Where("title LIKE ? OR content LIKE ? OR tags LIKE ?", like, like, like)
	}
	if err := db.Order("pinned DESC").Order("updated_at DESC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("memory: 查询记忆失败: %w", err)
	}
	return items, nil
}

func Create(ctx context.Context, studentID string, req dto.MemoryRequest) (*model.MemoryItem, error) {
	item := buildItem(studentID, req)
	if err := validateItem(item); err != nil {
		return nil, err
	}
	if err := dao.DB.WithContext(ctx).Create(item).Error; err != nil {
		return nil, fmt.Errorf("memory: 创建记忆失败: %w", err)
	}
	return item, nil
}

func Update(ctx context.Context, studentID, id string, req dto.MemoryRequest) (*model.MemoryItem, error) {
	var item model.MemoryItem
	err := dao.DB.WithContext(ctx).Where("id = ? AND student_id = ?", id, studentID).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrMemoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("memory: 查询记忆失败: %w", err)
	}
	next := buildItem(studentID, req)
	next.ID = item.ID
	if err := validateItem(next); err != nil {
		return nil, err
	}
	updates := map[string]any{
		"type":            next.Type,
		"title":           next.Title,
		"content":         next.Content,
		"tags":            next.Tags,
		"source_paper_id": next.SourcePaperID,
		"pinned":          next.Pinned,
	}
	if err := dao.DB.WithContext(ctx).Model(&item).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("memory: 更新记忆失败: %w", err)
	}
	if err := dao.DB.WithContext(ctx).Where("id = ? AND student_id = ?", id, studentID).First(&item).Error; err != nil {
		return nil, fmt.Errorf("memory: 读取更新后记忆失败: %w", err)
	}
	return &item, nil
}

func Delete(ctx context.Context, studentID, id string) error {
	res := dao.DB.WithContext(ctx).Where("id = ? AND student_id = ?", id, studentID).Delete(&model.MemoryItem{})
	if res.Error != nil {
		return fmt.Errorf("memory: 删除记忆失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrMemoryNotFound
	}
	return nil
}

func buildItem(studentID string, req dto.MemoryRequest) *model.MemoryItem {
	t := strings.TrimSpace(req.Type)
	if t == "" {
		t = "论文笔记"
	}
	return &model.MemoryItem{
		StudentID:     studentID,
		Type:          t,
		Title:         strings.TrimSpace(req.Title),
		Content:       strings.TrimSpace(req.Content),
		Tags:          normalizeTags(req.Tags),
		SourcePaperID: strings.TrimSpace(req.SourcePaperID),
		Pinned:        req.Pinned,
	}
}

func validateItem(item *model.MemoryItem) error {
	if item.Title == "" || item.Content == "" || item.Type == "" {
		return errs.ErrMemoryInvalid
	}
	if utf8.RuneCountInString(item.Type) > maxTypeRunes ||
		utf8.RuneCountInString(item.Title) > maxTitleRunes ||
		utf8.RuneCountInString(item.Content) > maxContentRunes {
		return errs.ErrMemoryInvalid
	}
	return nil
}

func normalizeTags(tags []string) model.JSONStrings {
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		rs := []rune(tag)
		if len(rs) > maxTagRunes {
			tag = string(rs[:maxTagRunes])
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
		if len(out) >= maxTags {
			break
		}
	}
	return model.JSONStrings(out)
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	return strings.ReplaceAll(s, "_", "\\_")
}
