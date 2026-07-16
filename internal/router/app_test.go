package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"GopherPaper/internal/auth"
	"GopherPaper/internal/config"
	"GopherPaper/internal/dao"
)

// TestAPIOnlyRouter 校验 Gin 只提供 API、健康检查和 Swagger 文档,前端由 Next 独立承载。
func TestAPIOnlyRouter(t *testing.T) {
	auth.Init(config.JWTConfig{EnableDebugToken: false})
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

func TestDebugTokenRouteDisabledByDefault(t *testing.T) {
	auth.Init(config.JWTConfig{EnableDebugToken: false})
	r := Init(gin.TestMode)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("debug token route status = %d, want 404", w.Code)
	}
}

func TestDebugTokenRouteNeverRegisteredInRelease(t *testing.T) {
	auth.Init(config.JWTConfig{EnableDebugToken: true})
	t.Cleanup(func() { auth.Init(config.JWTConfig{}) })
	r := Init(gin.ReleaseMode)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("release debug token route status = %d, want 404", w.Code)
	}
}

func TestDebugTokenRejectsNonLoopbackClient(t *testing.T) {
	auth.Init(config.JWTConfig{EnableDebugToken: true})
	t.Cleanup(func() { auth.Init(config.JWTConfig{}) })
	r := Init(gin.TestMode)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("remote debug token status = %d, want 404", w.Code)
	}
}

func TestDebugTokenEnabledForLoopbackTestRequest(t *testing.T) {
	server := miniredis.RunT(t)
	oldRDB := dao.RDB
	dao.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = dao.RDB.Close()
		dao.RDB = oldRDB
		auth.Init(config.JWTConfig{})
	})
	auth.Init(config.JWTConfig{
		Secret:           "test-secret-at-least-32-characters",
		Issuer:           "gopherpaper-test",
		ExpireHours:      1,
		EnableDebugToken: true,
	})
	r := Init(gin.TestMode)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/token",
		strings.NewReader(`{"student_id":"20260001","class_id":"class-a"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("loopback debug token status = %d, body = %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != auth.UserCookieName || !cookies[0].HttpOnly {
		t.Fatalf("debug token cookie = %#v", cookies)
	}
}
