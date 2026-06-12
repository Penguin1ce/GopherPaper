// Package credential 承载请求级的第三方凭据。凭据由前端保存、随请求头透传,
// 服务端只经 context 流到 mcp 工具调用处按次注入,不落库不打日志,与 tenant 的身份上下文平行。
package credential

import "context"

type ctxKey string

// With 把某 provider 的凭据注入 context,provider 或 token 为空时原样返回。
func With(ctx context.Context, provider, token string) context.Context {
	if provider == "" || token == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey(provider), token)
}

// From 取某 provider 的凭据,缺失返回空串。
func From(ctx context.Context, provider string) string {
	if v, ok := ctx.Value(ctxKey(provider)).(string); ok {
		return v
	}
	return ""
}
