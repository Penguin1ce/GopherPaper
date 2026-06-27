// Package user 处理用户注册、登录、登出、邮箱验证码与头像接口。
package user

import (
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
	"GopherPaper/internal/response"
	userservice "GopherPaper/internal/service/user"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

// SendCode 下发邮箱验证码。
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

// Login 校验学号密码并签发 JWT。
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
		AvatarURL: user.AvatarURL,
		ClassID:   user.ClassID,
	})
}

// SendPasswordResetCode 向绑定邮箱下发找回密码验证码。
func SendPasswordResetCode(c *gin.Context) {
	var req dto.PasswordResetCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	if err := userservice.SendPasswordResetCode(c.Request.Context(), req); err != nil {
		zlog.Error("发送找回密码验证码失败", "student_id", req.StudentID, "email", req.Email, "err", err)
		response.Fail(c, http.StatusInternalServerError, "验证码发送失败")
		return
	}
	response.OKMsg(c, "如果账号与邮箱匹配，验证码已发送", nil)
}

// ResetPassword 校验找回密码验证码并更新密码。
func ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	if err := userservice.ResetPassword(c.Request.Context(), req); err != nil {
		switch {
		case errors.Is(err, errs.ErrCodeExpired), errors.Is(err, errs.ErrCodeMismatch):
			response.Fail(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, errs.ErrUserNotFound):
			response.Fail(c, http.StatusBadRequest, "账号与邮箱不匹配")
		default:
			zlog.Error("重置密码失败", "student_id", req.StudentID, "email", req.Email, "err", err)
			response.Fail(c, http.StatusInternalServerError, "重置密码失败")
		}
		return
	}
	ai.EvictUser(req.StudentID)
	response.OKMsg(c, "密码已重置，请重新登录", nil)
}

// Me 返回当前登录用户资料。
func Me(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	if studentID == "" {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	user, err := userservice.Profile(c.Request.Context(), studentID)
	if err != nil {
		writeProfileErr(c, err)
		return
	}
	response.OK(c, profileResponse(user))
}

func UpdateProfile(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	if studentID == "" {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	user, err := userservice.UpdateProfile(c.Request.Context(), studentID, req)
	if err != nil {
		writeProfileErr(c, err)
		return
	}
	response.OK(c, profileResponse(user))
}

func UpdateEmail(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	if studentID == "" {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	var req dto.UpdateEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	user, err := userservice.UpdateEmail(c.Request.Context(), studentID, req)
	if err != nil {
		writeProfileErr(c, err)
		return
	}
	ai.EvictUser(studentID)
	response.OK(c, profileResponse(user))
}

// Logout 注销当前登录状态。
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

// UploadAvatar 更新当前用户头像。图片由前端裁剪后上传，后端负责格式、大小与归属校验。
// POST /api/v1/user/avatar
func UploadAvatar(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	if studentID == "" {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, constant.MaxAvatarBytes+(512<<10))
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "缺少上传图片 file")
		return
	}
	if fileHeader.Size > constant.MaxAvatarBytes {
		response.Fail(c, http.StatusRequestEntityTooLarge, "头像图片不能超过 2MB")
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "读取头像失败")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, constant.MaxAvatarBytes+1))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "读取头像失败")
		return
	}
	avatarURL, err := userservice.UpdateAvatar(c.Request.Context(), studentID, data)
	if err != nil {
		writeAvatarErr(c, err)
		return
	}
	response.OK(c, dto.AvatarResponse{AvatarURL: avatarURL})
}

// ClearAvatar 恢复默认首字母头像。
// DELETE /api/v1/user/avatar
func ClearAvatar(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	if studentID == "" {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	if err := userservice.ClearAvatar(c.Request.Context(), studentID); err != nil {
		writeAvatarErr(c, err)
		return
	}
	response.OK(c, dto.AvatarResponse{})
}

// AvatarFile 返回已保存头像文件。头像本身不是敏感数据，读取接口不要求 Authorization。
// GET /api/v1/user/avatar-files/:name
func AvatarFile(c *gin.Context) {
	path, ok := userservice.AvatarFilePath(c.Param("name"))
	if !ok {
		response.Fail(c, http.StatusBadRequest, "非法文件名")
		return
	}
	if _, err := os.Stat(path); err != nil {
		response.Fail(c, http.StatusNotFound, "头像不存在")
		return
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.File(path)
}

func profileResponse(user *model.User) dto.UserProfileResponse {
	return dto.UserProfileResponse{
		StudentID: user.StudentID,
		Name:      user.Name,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
		ClassID:   user.ClassID,
	}
}

func writeProfileErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errs.ErrCodeExpired), errors.Is(err, errs.ErrCodeMismatch):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, errs.ErrUserExists):
		response.Fail(c, http.StatusBadRequest, "邮箱已被其他账号绑定")
	case errors.Is(err, errs.ErrUserNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	default:
		zlog.Error("用户资料接口错误", "err", err)
		response.Fail(c, http.StatusInternalServerError, "用户资料更新失败")
	}
}

func writeAvatarErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errs.ErrAvatarInvalid):
		response.Fail(c, http.StatusBadRequest, "仅支持 jpg、png、webp 图片")
	case errors.Is(err, errs.ErrAvatarTooLarge):
		response.Fail(c, http.StatusRequestEntityTooLarge, "头像图片不能超过 2MB")
	case errors.Is(err, errs.ErrUserNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	default:
		zlog.Error("头像接口错误", "err", err)
		response.Fail(c, http.StatusInternalServerError, "头像更新失败")
	}
}
