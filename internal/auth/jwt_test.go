package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"GopherPaper/internal/config"
	"GopherPaper/internal/dao"
	"GopherPaper/pkg/errs"
)

func useMemorySessions(t *testing.T, debug bool) map[string]string {
	t.Helper()

	oldSecret := append([]byte(nil), secret...)
	oldIssuer, oldExpire, oldDebug := issuer, expire, enableDebugToken
	oldSetTTL, oldGet := setTTL, getValue
	oldGetDel, oldDel := getDelValue, delValue

	values := make(map[string]string)
	setTTL = func(_ context.Context, key string, value any, _ time.Duration) error {
		values[key] = fmt.Sprint(value)
		return nil
	}
	getValue = func(_ context.Context, key string) (string, error) {
		value, ok := values[key]
		if !ok {
			return "", dao.ErrCacheMiss
		}
		return value, nil
	}
	getDelValue = func(_ context.Context, key string) (string, error) {
		value, ok := values[key]
		if !ok {
			return "", dao.ErrCacheMiss
		}
		delete(values, key)
		return value, nil
	}
	delValue = func(_ context.Context, keys ...string) (int64, error) {
		var deleted int64
		for _, key := range keys {
			if _, ok := values[key]; ok {
				delete(values, key)
				deleted++
			}
		}
		return deleted, nil
	}
	Init(config.JWTConfig{
		Secret:           "test-secret-at-least-32-characters",
		Issuer:           "gopherpaper-test",
		ExpireHours:      1,
		EnableDebugToken: debug,
	})

	t.Cleanup(func() {
		secret = oldSecret
		issuer, expire, enableDebugToken = oldIssuer, oldExpire, oldDebug
		setTTL, getValue = oldSetTTL, oldGet
		getDelValue, delValue = oldGetDel, oldDel
	})
	return values
}

func TestParseRejectsNonHS256(t *testing.T) {
	useMemorySessions(t, false)
	now := time.Now()

	userToken := jwt.NewWithClaims(jwt.SigningMethodHS384, Claims{
		StudentID: "20260001",
		RegisteredClaims: jwt.RegisteredClaims{
			ID: "user-jti", Issuer: issuer,
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	})
	userRaw, err := userToken.SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(userRaw); !errors.Is(err, errs.ErrInvalidToken) {
		t.Fatalf("Parse(HS384) error = %v, want ErrInvalidToken", err)
	}

	adminToken := jwt.NewWithClaims(jwt.SigningMethodHS512, AdminClaims{
		AdminID: 1, Username: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ID: "admin-jti", Issuer: issuer,
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	})
	adminRaw, err := adminToken.SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAdmin(adminRaw); !errors.Is(err, errs.ErrInvalidToken) {
		t.Fatalf("ParseAdmin(HS512) error = %v, want ErrInvalidToken", err)
	}
}

func TestUserSessionLifecycle(t *testing.T) {
	values := useMemorySessions(t, false)
	ctx := context.Background()

	first, err := Issue(ctx, "20260001", "class-a")
	if err != nil {
		t.Fatal(err)
	}
	firstClaims, err := Parse(first)
	if err != nil {
		t.Fatal(err)
	}
	if firstClaims.ID == "" {
		t.Fatal("签发的 JWT 缺少 jti")
	}
	if err := ValidateSession(ctx, first, firstClaims); err != nil {
		t.Fatalf("首次签发的会话应有效: %v", err)
	}
	stored := values[userSessionKey("20260001")]
	if stored == first || len(stored) != 64 {
		t.Fatalf("Redis 应只保存 SHA-256 指纹, got %q", stored)
	}

	second, err := Issue(ctx, "20260001", "class-a")
	if err != nil {
		t.Fatal(err)
	}
	secondClaims, err := Parse(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstClaims.ID == secondClaims.ID {
		t.Fatal("重新登录必须生成新的 jti")
	}
	if err := ValidateSession(ctx, first, firstClaims); !errors.Is(err, errs.ErrInvalidToken) {
		t.Fatalf("重新登录后旧会话仍有效: %v", err)
	}
	if err := ValidateSession(ctx, second, secondClaims); err != nil {
		t.Fatalf("新会话应有效: %v", err)
	}

	if err := RevokeUser(ctx, "20260001"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSession(ctx, second, secondClaims); !errors.Is(err, errs.ErrInvalidToken) {
		t.Fatalf("撤销后会话仍有效: %v", err)
	}
}

func TestAdminSessionLifecycle(t *testing.T) {
	useMemorySessions(t, false)
	ctx := context.Background()

	raw, err := IssueAdmin(ctx, 7, "root")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseAdmin(raw)
	if err != nil {
		t.Fatal(err)
	}
	if claims.ID == "" {
		t.Fatal("签发的管理员 JWT 缺少 jti")
	}
	if err := ValidateAdminSession(ctx, raw, claims); err != nil {
		t.Fatalf("管理员会话应有效: %v", err)
	}
	if err := RevokeAdmin(ctx, 7); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAdminSession(ctx, raw, claims); !errors.Is(err, errs.ErrInvalidToken) {
		t.Fatalf("撤销后管理员会话仍有效: %v", err)
	}
}

func TestSSETicketCanOnlyBeConsumedOnce(t *testing.T) {
	useMemorySessions(t, false)
	ctx := context.Background()

	ticket, err := MintSSETicket(ctx, "20260001")
	if err != nil {
		t.Fatal(err)
	}
	studentID, err := ConsumeSSETicket(ctx, ticket)
	if err != nil {
		t.Fatal(err)
	}
	if studentID != "20260001" {
		t.Fatalf("studentID = %q", studentID)
	}
	if _, err := ConsumeSSETicket(ctx, ticket); !errors.Is(err, errs.ErrInvalidToken) {
		t.Fatalf("重复消费 SSE 票据 error = %v, want ErrInvalidToken", err)
	}
	if _, err := ConsumeSSETicket(ctx, "not-a-ticket"); !errors.Is(err, errs.ErrInvalidToken) {
		t.Fatalf("非法 SSE 票据 error = %v, want ErrInvalidToken", err)
	}
}

func TestDebugTokenAllowed(t *testing.T) {
	useMemorySessions(t, true)
	tests := []struct {
		name                   string
		mode, remote, clientIP string
		want                   bool
	}{
		{name: "debug ipv4 loopback", mode: "debug", remote: "127.0.0.1:1234", clientIP: "127.0.0.1", want: true},
		{name: "test ipv6 loopback", mode: "test", remote: "[::1]:1234", clientIP: "::1", want: true},
		{name: "release disabled", mode: "release", remote: "127.0.0.1:1234", clientIP: "127.0.0.1"},
		{name: "remote client", mode: "debug", remote: "192.0.2.10:1234", clientIP: "192.0.2.10"},
		{name: "reverse proxy external client", mode: "debug", remote: "127.0.0.1:1234", clientIP: "192.0.2.10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DebugTokenAllowed(tt.mode, tt.remote, tt.clientIP); got != tt.want {
				t.Fatalf("DebugTokenAllowed() = %v, want %v", got, tt.want)
			}
		})
	}

	enableDebugToken = false
	if DebugTokenAllowed("debug", "127.0.0.1:1234", "127.0.0.1") {
		t.Fatal("未显式开启时不应允许调试 Token")
	}
}

func TestSessionCookiesAreHttpOnly(t *testing.T) {
	useMemorySessions(t, false)
	req := httptest.NewRequest("POST", "https://example.test/api/v1/user/login", nil)
	recorder := httptest.NewRecorder()
	SetUserCookie(recorder, req, "secret-token")

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Set-Cookie count = %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != UserCookieName || cookie.Value != "secret-token" {
		t.Fatalf("unexpected cookie: %#v", cookie)
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.Path != "/api/v1" {
		t.Fatalf("cookie security attributes invalid: %#v", cookie)
	}
	if !strings.Contains(recorder.Header().Get("Set-Cookie"), "SameSite=Lax") {
		t.Fatalf("Set-Cookie missing SameSite=Lax: %s", recorder.Header().Get("Set-Cookie"))
	}
}
