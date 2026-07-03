package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	adminservice "GopherPaper/internal/service/admin"
)

// AdminAudit 记录管理员的写操作(POST/PUT/DELETE/PATCH),读操作不记。
// 须挂在 AdminJWTAuth 之后,以便从 context 读取管理员身份。
func AdminAudit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return
		}

		var adminID uint
		if v, ok := c.Get(AdminIDKey); ok {
			if id, ok := v.(uint); ok {
				adminID = id
			}
		}
		var adminName string
		if v, ok := c.Get(AdminUsernameKey); ok {
			if name, ok := v.(string); ok {
				adminName = name
			}
		}

		adminservice.RecordAudit(
			c.Request.Context(),
			adminID,
			adminName,
			c.Request.Method,
			c.FullPath(),
			c.ClientIP(),
			c.Writer.Status(),
		)
	}
}
