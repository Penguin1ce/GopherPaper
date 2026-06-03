package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"GopherCPP/internal/auth"
	"GopherCPP/internal/dto"
	"GopherCPP/internal/response"
	"GopherCPP/internal/zlog"
)

// Token 按 student_id 签发 JWT，公开接口。
// POST /api/v1/auth/token
func Token(c *gin.Context) {
	var req dto.TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	token, err := auth.Generate(req.StudentID, req.ClassID)
	if err != nil {
		zlog.Error("签发 token 失败", "err", err)
		response.Fail(c, http.StatusInternalServerError, "签发失败")
		return
	}
	response.OK(c, dto.TokenResponse{Token: token})
}
