// Package tenant 承载多租户上下文。每个学生是一个租户，StudentID 经 context
// 一路透传到 RAG 检索层，用于知识库隔离。
package tenant

import (
	"context"

	"GopherPaper/pkg/errs"
)

type ctxKey struct{}

// Tenant 描述一次请求归属的学生身份。
type Tenant struct {
	StudentID string // 私有知识库的隔离键
	ClassID   string // 班级，预留按班级共享等扩展
}

// With 把租户信息注入 context。
func With(ctx context.Context, t Tenant) context.Context {
	return context.WithValue(ctx, ctxKey{}, t)
}

// From 从 context 取出租户信息。
func From(ctx context.Context) (Tenant, error) {
	t, ok := ctx.Value(ctxKey{}).(Tenant)
	if !ok || t.StudentID == "" {
		return Tenant{}, errs.ErrNoTenant
	}
	return t, nil
}

// MustStudentID 取学生 ID，缺失时返回空串。
func MustStudentID(ctx context.Context) string {
	if t, err := From(ctx); err == nil {
		return t.StudentID
	}
	return ""
}
