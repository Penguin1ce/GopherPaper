package auth

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"GopherPaper/pkg/errs"
)

type AdminClaims struct {
	AdminID  uint   `json:"aid"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func GenerateAdmin(adminID uint, username string) (string, error) {
	now := time.Now()
	claims := AdminClaims{
		AdminID:  adminID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "admin:" + strconv.FormatUint(uint64(adminID), 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expire)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func ParseAdmin(tokenStr string) (*AdminClaims, error) {
	var claims AdminClaims
	_, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	}, jwt.WithIssuer(issuer))
	if err != nil || claims.AdminID == 0 || claims.Username == "" {
		return nil, errs.ErrInvalidToken
	}
	return &claims, nil
}
