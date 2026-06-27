// Package user 处理用户注册、登录与邮箱验证码下发。
// 处理函数为裸包级 func，业务委托给 service，本层只做参数绑定与错误映射。
package user

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/response"
	userservice "GopherPaper/internal/service/user"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/errs"
)

// SendCode 下发邮箱验证码，有效期 5 分钟。
// POST /api/v1/user/send-code
//
// @Summary 下发邮箱验证码
// @Description 向注册邮箱发送 6 位验证码，验证码有效期 5 分钟。
// @Tags user
// @Accept json
// @Produce json
// @Param request body dto.SendCodeRequest true "邮箱"
// @Success 200 {object} dto.Response
// @Failure 400 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /user/send-code [post]
func SendCode(c *gin.Context) {
	var req dto.SendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	if err := userservice.SendVerifyCode(c.Request.Context(), req.Email); err != nil {
		zlog.Error("发送验证码失败", "email", req.Email, "err", err)
		response.Fail(c, http.StatusInternalServerError, "验证码发送失败")
		return
	}
	response.OKMsg(c, "验证码已发送", nil)
}

// Register 校验邮箱验证码并注册用户。
// POST /api/v1/user/register
//
// @Summary 注册用户
// @Description 校验邮箱验证码后创建学生用户。
// @Tags user
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "注册参数"
// @Success 200 {object} dto.Response
// @Failure 400 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /user/register [post]
func Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	if err := userservice.Register(c.Request.Context(), req); err != nil {
		switch {
		case errors.Is(err, errs.ErrCodeExpired),
			errors.Is(err, errs.ErrCodeMismatch),
			errors.Is(err, errs.ErrUserExists):
			response.Fail(c, http.StatusBadRequest, err.Error())
		default:
			zlog.Error("注册失败", "student_id", req.StudentID, "err", err)
			response.Fail(c, http.StatusInternalServerError, "注册失败")
		}
		return
	}
	response.OKMsg(c, "注册成功", nil)
}

// Login 校验学号密码并签发 JWT，token 同时写入 Redis。
// POST /api/v1/user/login
//
// @Summary 登录
// @Description 按学号和密码登录，返回 JWT 与用户基本信息。
// @Tags user
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "登录参数"
// @Success 200 {object} dto.Response{data=dto.LoginResponse}
// @Failure 400 {object} dto.Response
// @Failure 401 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /user/login [post]
func Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	token, user, err := userservice.Login(c.Request.Context(), req.StudentID, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrUserNotFound), errors.Is(err, errs.ErrWrongPassword):
			response.Fail(c, http.StatusUnauthorized, err.Error())
		default:
			zlog.Error("登录失败", "student_id", req.StudentID, "err", err)
			response.Fail(c, http.StatusInternalServerError, "登录失败")
		}
		return
	}
	response.OK(c, dto.LoginResponse{
		Token:     token,
		StudentID: user.StudentID,
		Name:      user.Name,
		Email:     user.Email,
	})
}

// Logout 注销当前登录:清除服务端登录态并释放该用户常驻的 agent runner 与模型缓存。
// 身份取自 JWT 注入的租户上下文,故须经鉴权中间件。
// POST /api/v1/user/logout
//
// @Summary 登出
// @Description 注销当前登录态，并释放该用户常驻的模型与 agent runner 缓存。
// @Tags user
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.Response
// @Failure 401 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /user/logout [post]
func Logout(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	if studentID == "" {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	if err := userservice.Logout(c.Request.Context(), studentID); err != nil {
		zlog.Error("登出失败", "student_id", studentID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "登出失败")
		return
	}
	ai.EvictUser(studentID)
	response.OKMsg(c, "已登出", nil)
}
