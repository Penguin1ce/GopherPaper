package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/response"
	adminservice "GopherPaper/internal/service/admin"
	"GopherPaper/internal/zlog"
)

// ── 存储管理 ─────────────────────────────────────────────────

func StorageOverview(c *gin.Context) {
	topN := parseInt(c.Query("top"), 10)
	res, err := adminservice.StorageOverview(c.Request.Context(), topN)
	if err != nil {
		zlog.Error("admin storage overview failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}
