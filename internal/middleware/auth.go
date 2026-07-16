// Package middleware 存放 gin 中间件。
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/auth"
	"GopherPaper/internal/response"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
)

// JWTAuth 校验 Bearer token 或 HttpOnly Cookie，把学生身份注入租户上下文供 RAG 隔离。
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok, fromCookie := requestToken(c, auth.UserCookieName)
		if !ok {
			response.Abort(c, http.StatusUnauthorized, "登录凭据缺失")
			return
		}
		claims, err := auth.Parse(token)
		if err != nil {
			if fromCookie {
				auth.ClearUserCookie(c.Writer, c.Request)
			}
			response.Abort(c, http.StatusUnauthorized, err.Error())
			return
		}
		if err := auth.ValidateSession(c.Request.Context(), token, claims); err != nil {
			if fromCookie {
				auth.ClearUserCookie(c.Writer, c.Request)
			}
			response.Abort(c, http.StatusUnauthorized, "登录已失效，请重新登录")
			return
		}
		ctx := tenant.With(c.Request.Context(), tenant.Tenant{
			StudentID: claims.StudentID,
			ClassID:   claims.ClassID,
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// requestToken 优先读取显式 Bearer，浏览器请求则回退到 HttpOnly Cookie。
func requestToken(c *gin.Context, cookieName string) (token string, ok, fromCookie bool) {
	if token, ok := bearer(c.GetHeader(constant.HeaderAuthorization)); ok {
		return token, true, false
	}
	token, err := c.Cookie(cookieName)
	if err != nil {
		return "", false, false
	}
	token = strings.TrimSpace(token)
	return token, token != "", true
}

// bearer 从 Bearer xxx 中提取 token，大小写不敏感。
func bearer(h string) (string, bool) {
	prefix := constant.BearerPrefix
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(h[len(prefix):])
	return token, token != ""
}
