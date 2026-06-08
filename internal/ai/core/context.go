package core

import "context"

type paperCtxKey struct{}

// WithPaperID 把当前问答围绕的论文 ID 注入 context，供检索限定到该论文。
func WithPaperID(ctx context.Context, paperID string) context.Context {
	return context.WithValue(ctx, paperCtxKey{}, paperID)
}

// PaperIDFrom 取出 context 中的论文 ID，缺失返回空串表示跨库问答。
func PaperIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(paperCtxKey{}).(string); ok {
		return v
	}
	return ""
}
