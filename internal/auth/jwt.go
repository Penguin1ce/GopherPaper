// Package auth 负责 JWT 的签发与校验，配置与函数均为包级。
// claims 携带学生身份，校验通过后由中间件注入租户上下文。
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"GopherCPP/internal/config"
	"GopherCPP/pkg/errs"
)

// Claims 是 JWT 载荷，标准声明加业务身份。
type Claims struct {
	StudentID string `json:"sid"`
	ClassID   string `json:"cid"`
	jwt.RegisteredClaims
}

var (
	secret []byte
	issuer string
	expire time.Duration
)

// Init 注入 JWT 配置，须在签发与校验前调用。
func Init(cfg config.JWTConfig) {
	secret = []byte(cfg.Secret)
	issuer = cfg.Issuer
	expire = time.Duration(cfg.ExpireHours) * time.Hour
}

// TTL 返回 token 有效期，供 Redis 存储对齐过期时间。
func TTL() time.Duration { return expire }

// Generate 为学生签发 token。
func Generate(studentID, classID string) (string, error) {
	now := time.Now()
	claims := Claims{
		StudentID: studentID,
		ClassID:   classID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   studentID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expire)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// Parse 校验签名与有效期并返回 claims。
func Parse(tokenStr string) (*Claims, error) {
	var claims Claims
	_, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		// 只接受 HMAC 签名，防止算法混淆攻击。
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	}, jwt.WithIssuer(issuer))
	if err != nil || claims.StudentID == "" {
		return nil, errs.ErrInvalidToken
	}
	return &claims, nil
}
