// Package user 是用户注册、登录与邮箱验证码下发的业务逻辑，包级函数直接读写 dao/auth/utils。
//
//	注册 Register: 校验 Redis 中的邮箱验证码 → 落库 → 删验证码
//	登录 Login:    校验学号密码 → 签发 JWT → 以邮箱前缀为键写入 Redis
package user

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"GopherPaper/internal/auth"
	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
	"GopherPaper/pkg/utils"
)

// SendVerifyCode 生成验证码存入 Redis 并发到邮箱，有效期见 constant.VerifyCodeTTL。
func SendVerifyCode(ctx context.Context, email string) error {
	code := genCode()
	if err := dao.SetTTL(ctx, codeKey(email), code, constant.VerifyCodeTTL); err != nil {
		return fmt.Errorf("service: 存储验证码失败: %w", err)
	}
	return utils.SendMail(email, code)
}

// Register 校验邮箱验证码后创建用户，成功即删除验证码。
func Register(ctx context.Context, req dto.RegisterRequest) error {
	code, err := dao.Get(ctx, codeKey(req.Email))
	if errors.Is(err, dao.ErrCacheMiss) {
		return errs.ErrCodeExpired
	}
	if err != nil {
		return fmt.Errorf("service: 读取验证码失败: %w", err)
	}
	if code != req.Code {
		return errs.ErrCodeMismatch
	}

	var count int64
	if err := dao.DB.WithContext(ctx).Model(&model.User{}).
		Where("student_id = ? OR email = ?", req.StudentID, req.Email).
		Count(&count).Error; err != nil {
		return fmt.Errorf("service: 查询用户失败: %w", err)
	}
	if count > 0 {
		return errs.ErrUserExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("service: 密码加密失败: %w", err)
	}
	user := &model.User{
		StudentID:    req.StudentID,
		Name:         req.Name,
		Email:        req.Email,
		ClassID:      req.ClassID,
		PasswordHash: string(hash),
	}
	if err := dao.DB.WithContext(ctx).Create(user).Error; err != nil {
		return fmt.Errorf("service: 创建用户失败: %w", err)
	}
	_, _ = dao.Del(ctx, codeKey(req.Email))
	return nil
}

// Login 校验学号与密码，签发 JWT 并以邮箱前缀为键写入 Redis，返回 token 与用户。
func Login(ctx context.Context, studentID, password string) (string, *model.User, error) {
	var user model.User
	err := dao.DB.WithContext(ctx).Where("student_id = ?", studentID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil, errs.ErrUserNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("service: 查询用户失败: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", nil, errs.ErrWrongPassword
	}

	token, err := auth.Generate(user.StudentID, user.ClassID)
	if err != nil {
		return "", nil, fmt.Errorf("service: 签发 token 失败: %w", err)
	}
	if err := dao.SetTTL(ctx, tokenKey(user.Email), token, auth.TTL()); err != nil {
		return "", nil, fmt.Errorf("service: 存储 token 失败: %w", err)
	}
	return token, &user, nil
}

func Profile(ctx context.Context, studentID string) (*model.User, error) {
	var user model.User
	err := dao.DB.WithContext(ctx).Where("student_id = ?", studentID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("service: 查询用户资料失败: %w", err)
	}
	return &user, nil
}

func UpdateProfile(ctx context.Context, studentID string, req dto.UpdateProfileRequest) (*model.User, error) {
	name := strings.TrimSpace(req.Name)
	if len([]rune(name)) > 64 {
		name = string([]rune(name)[:64])
	}

	var user model.User
	err := dao.DB.WithContext(ctx).Where("student_id = ?", studentID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("service: 查询用户资料失败: %w", err)
	}
	if err := dao.DB.WithContext(ctx).Model(&user).Update("name", name).Error; err != nil {
		return nil, fmt.Errorf("service: 更新用户资料失败: %w", err)
	}
	return Profile(ctx, studentID)
}

// Logout 清除该用户的登录态:按 studentID 查邮箱删除 Redis 中的 token。
// 用户不存在或 token 已不在视为已登出,不报错。常驻的 agent/模型缓存由 handler 另行清理。
func UpdateEmail(ctx context.Context, studentID string, req dto.UpdateEmailRequest) (*model.User, error) {
	nextEmail := strings.TrimSpace(req.Email)
	code, err := dao.Get(ctx, codeKey(nextEmail))
	if errors.Is(err, dao.ErrCacheMiss) {
		return nil, errs.ErrCodeExpired
	}
	if err != nil {
		return nil, fmt.Errorf("service: 读取邮箱验证码失败: %w", err)
	}
	if code != req.Code {
		return nil, errs.ErrCodeMismatch
	}

	var user model.User
	err = dao.DB.WithContext(ctx).Where("student_id = ?", studentID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("service: 查询用户资料失败: %w", err)
	}
	if strings.EqualFold(user.Email, nextEmail) {
		_, _ = dao.Del(ctx, codeKey(nextEmail))
		return &user, nil
	}

	var count int64
	if err := dao.DB.WithContext(ctx).Model(&model.User{}).
		Where("email = ? AND student_id <> ?", nextEmail, studentID).
		Count(&count).Error; err != nil {
		return nil, fmt.Errorf("service: 查询邮箱占用失败: %w", err)
	}
	if count > 0 {
		return nil, errs.ErrUserExists
	}

	oldEmail := user.Email
	if err := dao.DB.WithContext(ctx).Model(&user).Update("email", nextEmail).Error; err != nil {
		return nil, fmt.Errorf("service: 更新绑定邮箱失败: %w", err)
	}
	_, _ = dao.Del(ctx, codeKey(nextEmail))
	if oldEmail != "" {
		_, _ = dao.Del(ctx, tokenKey(oldEmail))
	}
	return Profile(ctx, studentID)
}

func Logout(ctx context.Context, studentID string) error {
	var user model.User
	err := dao.DB.WithContext(ctx).Select("email").Where("student_id = ?", studentID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("service: 查询用户失败: %w", err)
	}
	_, _ = dao.Del(ctx, tokenKey(user.Email))
	return nil
}

// genCode 生成 6 位数字验证码。
func genCode() string {
	const digits = "0123456789"
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = digits[int(b[i])%len(digits)]
	}
	return string(b)
}

// codeKey 验证码的 Redis 键，拼接完整邮箱。
func codeKey(email string) string { return constant.RedisKeyVerifyCode + email }

// tokenKey 登录 token 的 Redis 键，前缀取邮箱 @ 之前部分。
func tokenKey(email string) string { return constant.RedisKeyUserToken + emailPrefix(email) }

// emailPrefix 取邮箱 @ 之前的本地部分，无 @ 则返回原串。
func emailPrefix(email string) string {
	if i := strings.IndexByte(email, '@'); i > 0 {
		return email[:i]
	}
	return email
}
