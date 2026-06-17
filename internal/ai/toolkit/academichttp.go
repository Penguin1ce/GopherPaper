// toolkit academichttp.go 收口学术站请求的两件公共礼节:带描述性 User-Agent 表明身份,
// 以及撞 429/503 时优先听服务端 Retry-After 的退避建议。arXiv 官方就要求请求带 UA,
// 默认的 Go-http-client UA 易被各站当裸爬虫限流甚至直接挡(IEEE 的 418 即此类),
// 故所有学术 HTTP 请求统一经此设头与退避,而非各家硬刚反爬。
package toolkit

import (
	"net/http"
	"strconv"
	"time"
)

// academicUserAgent 是学术站请求统一的描述性 UA:标明来源与联系方式,按各站规矩表明身份。
const academicUserAgent = "GopherPaper/1.0 (academic paper assistant; mailto:love@muzimi.org)"

// setAcademicHeaders 给学术站请求挂统一 UA。
func setAcademicHeaders(req *http.Request) {
	req.Header.Set("User-Agent", academicUserAgent)
}

// retryAfterDelay 解析 429/503 响应的 Retry-After 头(秒数或 HTTP 日期)给出退避时长,
// 缺失或非法时回退到调用方传入的固定节奏。仅读头不动 Body,调用方仍负责关闭。
func retryAfterDelay(resp *http.Response, fallback time.Duration) time.Duration {
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return fallback
	}
	// 形式一:整数秒。
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return fallback
		}
		return time.Duration(secs) * time.Second
	}
	// 形式二:HTTP 日期,取与当下的差值。
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return fallback
}
