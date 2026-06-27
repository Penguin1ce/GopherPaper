package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/auth"
	"GopherPaper/internal/response"
	"GopherPaper/pkg/constant"
)

const (
	AdminIDKey       = "admin_id"
	AdminUsernameKey = "admin_username"
)

func AdminJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader(constant.HeaderAuthorization)
		token, ok := bearer(raw)
		if !ok {
			response.Abort(c, http.StatusUnauthorized, "Authorization header must be Bearer <token>")
			return
		}
		claims, err := auth.ParseAdmin(token)
		if err != nil {
			response.Abort(c, http.StatusUnauthorized, err.Error())
			return
		}
		c.Set(AdminIDKey, claims.AdminID)
		c.Set(AdminUsernameKey, claims.Username)
		c.Next()
	}
}
