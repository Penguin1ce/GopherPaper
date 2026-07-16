package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"GopherPaper/internal/dao"
	"GopherPaper/pkg/errs"
)

type AdminClaims struct {
	AdminID  uint   `json:"aid"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func generateAdmin(adminID uint, username string) (string, error) {
	now := time.Now()
	claims := AdminClaims{
		AdminID:  adminID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Issuer:    issuer,
			Subject:   "admin:" + strconv.FormatUint(uint64(adminID), 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expire)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// IssueAdmin 为管理员签发 JWT，并登记当前有效会话。
func IssueAdmin(ctx context.Context, adminID uint, username string) (string, error) {
	token, err := generateAdmin(adminID, username)
	if err != nil {
		return "", err
	}
	if err := setTTL(ctx, adminSessionKey(adminID), tokenFingerprint(token), expire); err != nil {
		return "", fmt.Errorf("auth: 保存管理员登录会话失败: %w", err)
	}
	return token, nil
}

func ParseAdmin(tokenStr string) (*AdminClaims, error) {
	var claims AdminClaims
	_, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	}, jwt.WithIssuer(issuer))
	if err != nil || claims.AdminID == 0 || claims.Username == "" {
		return nil, errs.ErrInvalidToken
	}
	return &claims, nil
}

func ValidateAdminSession(ctx context.Context, token string, claims *AdminClaims) error {
	if claims == nil || claims.AdminID == 0 || claims.ID == "" {
		return errs.ErrInvalidToken
	}
	want, err := getValue(ctx, adminSessionKey(claims.AdminID))
	if err != nil {
		if errors.Is(err, dao.ErrCacheMiss) {
			return errs.ErrInvalidToken
		}
		return fmt.Errorf("auth: 校验管理员登录会话失败: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(want), []byte(tokenFingerprint(token))) != 1 {
		return errs.ErrInvalidToken
	}
	return nil
}

func RevokeAdmin(ctx context.Context, adminID uint) error {
	_, err := delValue(ctx, adminSessionKey(adminID))
	return err
}
