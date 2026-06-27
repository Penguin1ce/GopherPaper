package user

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

var avatarExtByContentType = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

func UpdateAvatar(ctx context.Context, studentID string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", errs.ErrAvatarInvalid
	}
	if len(data) > constant.MaxAvatarBytes {
		return "", errs.ErrAvatarTooLarge
	}
	contentType := http.DetectContentType(data)
	ext, ok := avatarExtByContentType[contentType]
	if !ok {
		return "", errs.ErrAvatarInvalid
	}

	var current model.User
	err := dao.DB.WithContext(ctx).Select("id", "avatar_url").
		Where("student_id = ?", studentID).First(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", errs.ErrUserNotFound
	}
	if err != nil {
		return "", fmt.Errorf("service/user: query avatar owner failed: %w", err)
	}

	if err := os.MkdirAll(constant.AvatarStorageDir, 0o755); err != nil {
		return "", fmt.Errorf("service/user: create avatar dir failed: %w", err)
	}
	name := uuid.NewString() + ext
	path := filepath.Join(constant.AvatarStorageDir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("service/user: write avatar failed: %w", err)
	}

	nextURL := avatarURL(name)
	if err := dao.DB.WithContext(ctx).Model(&model.User{}).
		Where("student_id = ?", studentID).
		Update("avatar_url", nextURL).Error; err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("service/user: update avatar url failed: %w", err)
	}
	removeAvatarFile(current.AvatarURL)
	return nextURL, nil
}

func ClearAvatar(ctx context.Context, studentID string) error {
	var current model.User
	err := dao.DB.WithContext(ctx).Select("id", "avatar_url").
		Where("student_id = ?", studentID).First(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errs.ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("service/user: query avatar owner failed: %w", err)
	}
	if err := dao.DB.WithContext(ctx).Model(&model.User{}).
		Where("student_id = ?", studentID).
		Update("avatar_url", "").Error; err != nil {
		return fmt.Errorf("service/user: clear avatar url failed: %w", err)
	}
	removeAvatarFile(current.AvatarURL)
	return nil
}

func AvatarFilePath(name string) (string, bool) {
	base := filepath.Base(name)
	if base == "." || base == ".." || base == string(filepath.Separator) || base != name {
		return "", false
	}
	ext := strings.ToLower(filepath.Ext(base))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		return "", false
	}
	return filepath.Join(constant.AvatarStorageDir, base), true
}

func avatarURL(name string) string {
	return constant.AvatarURLPrefix + "/" + name
}

func removeAvatarFile(rawURL string) {
	if rawURL == "" {
		return
	}
	const prefix = constant.AvatarURLPrefix + "/"
	if !strings.HasPrefix(rawURL, prefix) {
		return
	}
	path, ok := AvatarFilePath(strings.TrimPrefix(rawURL, prefix))
	if !ok {
		return
	}
	_ = os.Remove(path)
}
