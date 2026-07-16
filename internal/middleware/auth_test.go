package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"GopherPaper/internal/auth"
	"GopherPaper/internal/config"
	"GopherPaper/internal/dao"
	"GopherPaper/internal/tenant"
)

func setupAuthMiddleware(t *testing.T) (string, *gin.Engine) {
	t.Helper()
	server := miniredis.RunT(t)
	oldRDB := dao.RDB
	dao.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = dao.RDB.Close()
		dao.RDB = oldRDB
	})

	auth.Init(config.JWTConfig{
		Secret:      "test-secret-at-least-32-characters",
		Issuer:      "gopherpaper-test",
		ExpireHours: 1,
	})
	token, err := auth.Issue(t.Context(), "20260001", "class-a")
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(JWTAuth())
	r.GET("/protected", func(c *gin.Context) {
		c.String(http.StatusOK, tenant.MustStudentID(c.Request.Context()))
	})
	return token, r
}

func TestJWTAuthAcceptsHttpOnlyCookie(t *testing.T) {
	token, r := setupAuthMiddleware(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.UserCookieName, Value: token})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "20260001" {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
}

func TestJWTAuthAcceptsBearerForExternalClients(t *testing.T) {
	token, r := setupAuthMiddleware(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "20260001" {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
}

func TestJWTAuthRejectsRevokedSession(t *testing.T) {
	token, r := setupAuthMiddleware(t)
	claims, err := auth.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.RevokeUser(t.Context(), claims.StudentID); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: auth.UserCookieName, Value: token})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if cookies := w.Result().Cookies(); len(cookies) != 1 || cookies[0].Name != auth.UserCookieName || cookies[0].MaxAge >= 0 {
		t.Fatalf("失效会话未清除浏览器 Cookie: %#v", cookies)
	}
}
