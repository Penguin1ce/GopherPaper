package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/auth"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/response"
	"GopherPaper/internal/zlog"
)

// Token 是仅供本地调试的 JWT 签发入口。
// POST /api/v1/auth/token
//
// @Summary 签发调试 JWT
// @Description 仅在 enable_debug_token=true、debug/test 模式且请求来自回环地址时可用。生产环境不注册该路由。
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.TokenRequest true "签发参数"
// @Success 200 {object} dto.Response{data=dto.TokenResponse}
// @Failure 400 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /auth/token [post]
func Token(c *gin.Context) {
	if !auth.DebugTokenAllowed(gin.Mode(), c.Request.RemoteAddr, c.ClientIP()) {
		response.Fail(c, http.StatusNotFound, "接口不存在")
		return
	}
	var req dto.TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	token, err := auth.Issue(c.Request.Context(), req.StudentID, req.ClassID)
	if err != nil {
		zlog.Error("签发 token 失败", "err", err)
		response.Fail(c, http.StatusInternalServerError, "签发失败")
		return
	}
	auth.SetUserCookie(c.Writer, c.Request, token)
	response.OK(c, dto.TokenResponse{Token: token})
}
