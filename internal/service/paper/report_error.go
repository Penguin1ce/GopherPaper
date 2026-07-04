package paper

import "strings"

// ReportErrorMessage maps low-level report generation errors to messages that
// can be shown directly in the UI while keeping full details in backend logs.
func ReportErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "token quota is not enough"),
		strings.Contains(msg, "pre_consume_token_quota_failed"),
		strings.Contains(msg, "insufficient_quota"),
		strings.Contains(msg, "quota exceeded"),
		strings.Contains(msg, "quota") && strings.Contains(msg, "not enough"),
		strings.Contains(msg, "403 forbidden") && strings.Contains(msg, "quota"):
		return "模型服务额度不足，当前账号余额不足以生成该研读报告。请充值或更换可用模型 API Key 后重试。"
	case strings.Contains(msg, "401 unauthorized"),
		strings.Contains(msg, "invalid api key"),
		strings.Contains(msg, "incorrect api key"),
		strings.Contains(msg, "authentication") && strings.Contains(msg, "failed"):
		return "模型服务认证失败，请检查模型 API Key 是否正确或已过期。"
	case strings.Contains(msg, "429"),
		strings.Contains(msg, "too many requests"),
		strings.Contains(msg, "rate limit"):
		return "模型服务请求过于频繁，请稍后重试或切换更高额度的模型配置。"
	case strings.Contains(msg, "context deadline exceeded"),
		strings.Contains(msg, "client.timeout"),
		strings.Contains(msg, "timeout"):
		return "模型服务响应超时，请稍后重试或检查模型网关是否可用。"
	default:
		return "报告生成失败，请稍后重试；如果持续失败，请检查模型服务配置和后端日志。"
	}
}
