package toolkit

import (
	"net/http"
	"testing"
	"time"
)

// TestRetryAfterDelay 覆盖 Retry-After 的秒数/日期/缺失/非法四种解析分支。
func TestRetryAfterDelay(t *testing.T) {
	fallback := 3 * time.Second

	mk := func(v string) *http.Response {
		h := http.Header{}
		if v != "" {
			h.Set("Retry-After", v)
		}
		return &http.Response{Header: h}
	}

	if got := retryAfterDelay(mk("10"), fallback); got != 10*time.Second {
		t.Errorf("整数秒应取 10s, 实际 %v", got)
	}
	if got := retryAfterDelay(mk(""), fallback); got != fallback {
		t.Errorf("缺失应回退 fallback, 实际 %v", got)
	}
	if got := retryAfterDelay(mk("not-a-number"), fallback); got != fallback {
		t.Errorf("非法值应回退 fallback, 实际 %v", got)
	}
	if got := retryAfterDelay(mk("0"), fallback); got != fallback {
		t.Errorf("非正秒数应回退 fallback, 实际 %v", got)
	}
	// HTTP 日期:取未来 5 秒,允许误差。
	future := time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat)
	if got := retryAfterDelay(mk(future), fallback); got < 3*time.Second || got > 6*time.Second {
		t.Errorf("HTTP 日期应约 5s, 实际 %v", got)
	}
	// 过去的日期不可用,回退 fallback。
	past := time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat)
	if got := retryAfterDelay(mk(past), fallback); got != fallback {
		t.Errorf("过去日期应回退 fallback, 实际 %v", got)
	}
}
