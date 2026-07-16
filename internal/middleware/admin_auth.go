package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/auth"
	"GopherPaper/internal/response"
)

const (
	AdminIDKey       = "admin_id"
	AdminUsernameKey = "admin_username"
)

func AdminJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok, fromCookie := requestToken(c, auth.AdminCookieName)
		if !ok {
			response.Abort(c, http.StatusUnauthorized, "admin login required")
			return
		}
		claims, err := auth.ParseAdmin(token)
		if err != nil {
			if fromCookie {
				auth.ClearAdminCookie(c.Writer, c.Request)
			}
			response.Abort(c, http.StatusUnauthorized, err.Error())
			return
		}
		if err := auth.ValidateAdminSession(c.Request.Context(), token, claims); err != nil {
			if fromCookie {
				auth.ClearAdminCookie(c.Writer, c.Request)
			}
			response.Abort(c, http.StatusUnauthorized, "admin login expired")
			return
		}
		c.Set(AdminIDKey, claims.AdminID)
		c.Set(AdminUsernameKey, claims.Username)
		c.Next()
	}
}
