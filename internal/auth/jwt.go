// Package auth 负责 JWT 的签发与校验，配置与函数均为包级。
// claims 携带学生身份，校验通过后由中间件注入租户上下文。
package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"GopherPaper/internal/config"
	"GopherPaper/internal/dao"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

// Claims 是 JWT 载荷，标准声明加业务身份。
type Claims struct {
	StudentID string `json:"sid"`
	ClassID   string `json:"cid"`
	jwt.RegisteredClaims
}

var (
	secret           []byte
	issuer           string
	expire           time.Duration
	enableDebugToken bool
	setTTL           = dao.SetTTL
	getValue         = dao.Get
	getDelValue      = dao.GetDel
	delValue         = dao.Del
)

const (
	UserCookieName  = "gopherpaper_session"
	AdminCookieName = "gopherpaper_admin_session"
	SSETicketTTL    = 30 * time.Second
)

// Init 注入 JWT 配置，须在签发与校验前调用。
func Init(cfg config.JWTConfig) {
	secret = []byte(cfg.Secret)
	issuer = cfg.Issuer
	expire = time.Duration(cfg.ExpireHours) * time.Hour
	enableDebugToken = cfg.EnableDebugToken
}

// generate 为学生生成带 jti 的 HS256 token，登记会话统一走 Issue。
func generate(studentID, classID string) (string, error) {
	now := time.Now()
	claims := Claims{
		StudentID: studentID,
		ClassID:   classID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Issuer:    issuer,
			Subject:   studentID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expire)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// Issue 为用户签发 JWT，并把当前有效会话写入 Redis。再次登录会使旧 token 失效。
func Issue(ctx context.Context, studentID, classID string) (string, error) {
	token, err := generate(studentID, classID)
	if err != nil {
		return "", err
	}
	if err := setTTL(ctx, userSessionKey(studentID), tokenFingerprint(token), expire); err != nil {
		return "", fmt.Errorf("auth: 保存用户登录会话失败: %w", err)
	}
	return token, nil
}

// Parse 校验签名与有效期并返回 claims。
func Parse(tokenStr string) (*Claims, error) {
	var claims Claims
	_, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		// 只接受 HS256，防止算法混淆及配置外算法被意外接受。
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	}, jwt.WithIssuer(issuer))
	if err != nil || claims.StudentID == "" {
		return nil, errs.ErrInvalidToken
	}
	return &claims, nil
}

// ValidateSession 确认 JWT 仍是该用户在 Redis 中登记的当前会话。
func ValidateSession(ctx context.Context, token string, claims *Claims) error {
	if claims == nil || claims.StudentID == "" || claims.ID == "" {
		return errs.ErrInvalidToken
	}
	want, err := getValue(ctx, userSessionKey(claims.StudentID))
	if err != nil {
		if errors.Is(err, dao.ErrCacheMiss) {
			return errs.ErrInvalidToken
		}
		return fmt.Errorf("auth: 校验用户登录会话失败: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(want), []byte(tokenFingerprint(token))) != 1 {
		return errs.ErrInvalidToken
	}
	return nil
}

func RevokeUser(ctx context.Context, studentID string) error {
	_, err := delValue(ctx, userSessionKey(studentID))
	return err
}

func DebugTokenEnabled() bool { return enableDebugToken }

// DebugTokenAllowed 要求显式开关，并限制在 debug/test 模式与回环来源。
// remoteAddr 与 clientAddr 同时校验，防止外部请求经本机反向代理后被误判为回环请求。
func DebugTokenAllowed(mode, remoteAddr, clientAddr string) bool {
	if !enableDebugToken || (mode != "debug" && mode != "test") {
		return false
	}
	return isLoopbackAddr(remoteAddr) && isLoopbackAddr(clientAddr)
}

func isLoopbackAddr(addr string) bool {
	addr = strings.TrimSpace(addr)
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func SetUserCookie(w http.ResponseWriter, r *http.Request, token string) {
	setCookie(w, r, UserCookieName, token, int(expire.Seconds()))
}

func ClearUserCookie(w http.ResponseWriter, r *http.Request) {
	setCookie(w, r, UserCookieName, "", -1)
}

func SetAdminCookie(w http.ResponseWriter, r *http.Request, token string) {
	setCookie(w, r, AdminCookieName, token, int(expire.Seconds()))
}

func ClearAdminCookie(w http.ResponseWriter, r *http.Request) {
	setCookie(w, r, AdminCookieName, "", -1)
}

func setCookie(w http.ResponseWriter, r *http.Request, name, value string, maxAge int) {
	secure := r != nil && (r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"))
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/api/v1", MaxAge: maxAge,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}

func MintSSETicket(ctx context.Context, studentID string) (string, error) {
	ticket := uuid.NewString()
	if err := setTTL(ctx, constant.RedisKeySSETicket+ticket, studentID, SSETicketTTL); err != nil {
		return "", fmt.Errorf("auth: 保存 SSE 票据失败: %w", err)
	}
	return ticket, nil
}

func ConsumeSSETicket(ctx context.Context, ticket string) (string, error) {
	ticket = strings.TrimSpace(ticket)
	if _, err := uuid.Parse(ticket); err != nil {
		return "", errs.ErrInvalidToken
	}
	studentID, err := getDelValue(ctx, constant.RedisKeySSETicket+ticket)
	if err != nil {
		if errors.Is(err, dao.ErrCacheMiss) {
			return "", errs.ErrInvalidToken
		}
		return "", fmt.Errorf("auth: 消费 SSE 票据失败: %w", err)
	}
	if strings.TrimSpace(studentID) == "" {
		return "", errs.ErrInvalidToken
	}
	return studentID, nil
}

func userSessionKey(studentID string) string {
	return constant.RedisKeyUserToken + studentID
}

func adminSessionKey(adminID uint) string {
	return constant.RedisKeyAdminToken + strconv.FormatUint(uint64(adminID), 10)
}

func tokenFingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum[:])
}
