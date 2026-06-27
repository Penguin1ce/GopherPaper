// Package errs 集中定义全项目共享的错误。
package errs

import "errors"

var (
	ErrNoTenant     = errors.New("tenant: 上下文中缺少租户信息")
	ErrInvalidToken = errors.New("auth: token 无效或已过期")

	ErrCodeExpired    = errors.New("auth: 验证码不存在或已过期")
	ErrCodeMismatch   = errors.New("auth: 验证码不正确")
	ErrUserExists     = errors.New("auth: 学号或邮箱已注册")
	ErrUserNotFound   = errors.New("auth: 用户不存在")
	ErrWrongPassword  = errors.New("auth: 密码错误")
	ErrAvatarInvalid  = errors.New("auth: invalid avatar image")
	ErrAvatarTooLarge = errors.New("auth: avatar image too large")

	ErrSessionNotFound  = errors.New("chat: 会话不存在")
	ErrSessionForbidden = errors.New("chat: 无权访问该会话")
	ErrAgentTypeInvalid = errors.New("chat: 不支持的会话 agent 类型")

	ErrPaperNotFound    = errors.New("paper: 论文不存在")
	ErrPaperForbidden   = errors.New("paper: 无权访问该论文")
	ErrInvalidFile      = errors.New("paper: 文件无效，仅支持 PDF")
	ErrParseFailed      = errors.New("paper: PDF 解析失败")
	ErrReportNotFound   = errors.New("paper: 研读报告缓存不存在")
	ErrReportGenerating = errors.New("paper: 研读报告正在生成中")
)
