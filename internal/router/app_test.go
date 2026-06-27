package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAPIOnlyRouter 校验 Gin 只提供 API、健康检查和 Swagger 文档,前端由 Next 独立承载。
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

	swagger := httptest.NewRecorder()
	r.ServeHTTP(swagger, httptest.NewRequest(http.MethodGet, "/swagger", nil))
	if swagger.Code != http.StatusMovedPermanently {
		t.Fatalf("/swagger status = %d", swagger.Code)
	}

	swaggerIndex := httptest.NewRecorder()
	r.ServeHTTP(swaggerIndex, httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))
	if swaggerIndex.Code != http.StatusOK {
		t.Fatalf("/swagger/index.html status = %d", swaggerIndex.Code)
	}

	doc := httptest.NewRecorder()
	r.ServeHTTP(doc, httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil))
	if doc.Code != http.StatusOK {
		t.Fatalf("/swagger/doc.json status = %d", doc.Code)
	}
	var spec map[string]any
	if err := json.Unmarshal(doc.Body.Bytes(), &spec); err != nil {
		t.Fatalf("/swagger/doc.json 返回非法 JSON: %v", err)
	}
	if spec["swagger"] != "2.0" {
		t.Fatal("/swagger/doc.json 缺少 swagger 2.0 字段")
	}
}
