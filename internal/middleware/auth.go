// Package middleware 存放 gin 中间件。
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"GopherCPP/internal/auth"
	"GopherCPP/internal/dto"
	"GopherCPP/internal/tenant"
	"GopherCPP/pkg/constant"
)

// JWTAuth 校验 Bearer token，把学生身份注入租户上下文供 RAG 隔离，失败返回 401。
func JWTAuth(mgr *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader(constant.HeaderAuthorization)
		token, ok := bearer(raw)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error: "Authorization 头缺失或格式错误，应为 Bearer <token>",
			})
			return
		}
		claims, err := mgr.Parse(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{Error: err.Error()})
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

// bearer 从 Bearer xxx 中提取 token，大小写不敏感。
func bearer(h string) (string, bool) {
	prefix := constant.BearerPrefix
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(h[len(prefix):])
	return token, token != ""
}
