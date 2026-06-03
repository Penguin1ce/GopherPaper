// Package errs 集中定义全项目共享的错误。
package errs

import "errors"

var (
	ErrNoTenant     = errors.New("tenant: 上下文中缺少租户信息")
	ErrInvalidToken = errors.New("auth: token 无效或已过期")

	ErrCodeExpired   = errors.New("auth: 验证码不存在或已过期")
	ErrCodeMismatch  = errors.New("auth: 验证码不正确")
	ErrUserExists    = errors.New("auth: 学号或邮箱已注册")
	ErrUserNotFound  = errors.New("auth: 用户不存在")
	ErrWrongPassword = errors.New("auth: 密码错误")
)
