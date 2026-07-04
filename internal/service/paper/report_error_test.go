package paper

import (
	"errors"
	"testing"
)

func TestReportErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "quota",
			err:  errors.New(`planstream: POST "https://api.example.com/v1/chat/completions": 403 Forbidden {"message":"token quota is not enough, token remain quota: 0.08, need quota: 0.10","code":"pre_consume_token_quota_failed"}`),
			want: "模型服务额度不足，当前账号余额不足以生成该研读报告。请充值或更换可用模型 API Key 后重试。",
		},
		{
			name: "auth",
			err:  errors.New("401 Unauthorized: invalid api key"),
			want: "模型服务认证失败，请检查模型 API Key 是否正确或已过期。",
		},
		{
			name: "rate limit",
			err:  errors.New("429 too many requests: rate limit reached"),
			want: "模型服务请求过于频繁，请稍后重试或切换更高额度的模型配置。",
		},
		{
			name: "timeout",
			err:  errors.New("context deadline exceeded"),
			want: "模型服务响应超时，请稍后重试或检查模型网关是否可用。",
		},
		{
			name: "default",
			err:  errors.New("unexpected provider error"),
			want: "报告生成失败，请稍后重试；如果持续失败，请检查模型服务配置和后端日志。",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ReportErrorMessage(tt.err); got != tt.want {
				t.Fatalf("ReportErrorMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
