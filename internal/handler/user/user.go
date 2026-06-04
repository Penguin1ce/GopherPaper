// Package user 处理用户注册、登录与邮箱验证码下发。
// 处理函数为裸包级 func，业务委托给 service，本层只做参数绑定与错误映射。
package user

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/dto"
	"GopherPaper/internal/response"
	userservice "GopherPaper/internal/service/user"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/errs"
)

// SendCode 下发邮箱验证码，有效期 5 分钟。
// POST /api/v1/user/send-code
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
