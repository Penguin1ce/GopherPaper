// Package auth 负责 JWT 的签发与校验。claims 携带学生身份，
// 校验通过后由中间件注入租户上下文。
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"GopherCPP/internal/config"
)

var ErrInvalidToken = errors.New("auth: token 无效或已过期")

// Claims 是 JWT 载荷，标准声明加业务身份。
type Claims struct {
	StudentID string `json:"sid"`
	ClassID   string `json:"cid"`
	jwt.RegisteredClaims
}

// Manager 用 HS256 对称密钥签发与校验 token。
type Manager struct {
	secret []byte
	issuer string
	expire time.Duration
}

func NewManager(cfg config.JWTConfig) *Manager {
	return &Manager{
		secret: []byte(cfg.Secret),
		issuer: cfg.Issuer,
		expire: time.Duration(cfg.ExpireHours) * time.Hour,
	}
}

// Generate 为学生签发 token。
func (m *Manager) Generate(studentID, classID string) (string, error) {
	now := time.Now()
	claims := Claims{
		StudentID: studentID,
		ClassID:   classID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   studentID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expire)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// Parse 校验签名与有效期并返回 claims。
func (m *Manager) Parse(tokenStr string) (*Claims, error) {
	var claims Claims
	_, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		// 只接受 HMAC 签名，防止算法混淆攻击。
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer))
	if err != nil || claims.StudentID == "" {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}
