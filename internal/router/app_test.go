package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAPIOnlyRouter 校验 Gin 只提供 API 与健康检查,前端由 Next 独立承载。
func TestAPIOnlyRouter(t *testing.T) {
	r := Init("test")

	health := httptest.NewRecorder()
	r.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("/healthz status = %d", health.Code)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("/ status = %d", w.Code)
	}
}
