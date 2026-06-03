// Package errs 集中定义全项目共享的错误。
package errs

import "errors"

var (
	ErrNoTenant     = errors.New("tenant: 上下文中缺少租户信息")
	ErrInvalidToken = errors.New("auth: token 无效或已过期")
)
