package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/auth"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/response"
	"GopherPaper/internal/zlog"
)

// Token 按 student_id 签发 JWT，公开接口。
// POST /api/v1/auth/token
//
// @Summary 签发调试 JWT
// @Description 按 student_id 和 class_id 签发 JWT，主要用于本地调试或外部工具联调。
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.TokenRequest true "签发参数"
// @Success 200 {object} dto.Response{data=dto.TokenResponse}
// @Failure 400 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /auth/token [post]
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
