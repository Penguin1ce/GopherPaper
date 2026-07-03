package admin

import (
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/dto"
	"GopherPaper/internal/middleware"
	"GopherPaper/internal/response"
	adminservice "GopherPaper/internal/service/admin"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/errs"
)

func SendCode(c *gin.Context) {
	var req dto.SendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if err := adminservice.SendVerifyCode(c.Request.Context(), req.Email); err != nil {
		zlog.Error("admin send code failed", "email", req.Email, "err", err)
		response.Fail(c, http.StatusInternalServerError, "verification code send failed")
		return
	}
	response.OKMsg(c, "verification code sent", nil)
}

func Register(c *gin.Context) {
	var req dto.AdminRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	admin, err := adminservice.Register(c.Request.Context(), req)
	if err != nil {
		writeAdminErr(c, err, "register failed")
		return
	}
	response.OK(c, adminservice.Profile(admin))
}

func Login(c *gin.Context) {
	var req dto.AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		email = strings.TrimSpace(req.Account)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: email is required")
		return
	}
	token, admin, err := adminservice.Login(c.Request.Context(), email, req.Password)
	if err != nil {
		writeAdminErr(c, err, "login failed")
		return
	}
	response.OK(c, dto.AdminLoginResponse{Token: token, Admin: adminservice.Profile(admin)})
}

func Me(c *gin.Context) {
	adminID, ok := currentAdminID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "not logged in")
		return
	}
	admin, err := adminservice.Get(c.Request.Context(), adminID)
	if err != nil {
		writeAdminErr(c, err, "query failed")
		return
	}
	response.OK(c, adminservice.Profile(admin))
}

func Overview(c *gin.Context) {
	overview, err := adminservice.Overview(c.Request.Context())
	if err != nil {
		zlog.Error("admin overview failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, overview)
}

func ListPapers(c *gin.Context) {
	page := parseInt(c.Query("page"), 1)
	pageSize := parseInt(c.Query("page_size"), 10)
	res, err := adminservice.ListPapers(
		c.Request.Context(),
		c.Query("query"),
		c.Query("status"),
		c.Query("date_range"),
		page,
		pageSize,
	)
	if err != nil {
		zlog.Error("admin list papers failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

func DeletePaper(c *gin.Context) {
	if err := adminservice.DeletePaper(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, errs.ErrPaperNotFound) {
			response.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		zlog.Error("admin delete paper failed", "paper_id", c.Param("id"), "err", err)
		response.Fail(c, http.StatusInternalServerError, "delete failed")
		return
	}
	response.OK(c, nil)
}

func ListModelConfigs(c *gin.Context) {
	res, err := adminservice.ListModelConfigs(c.Request.Context())
	if err != nil {
		writeAdminErr(c, err, "query model configs failed")
		return
	}
	response.OK(c, res)
}

func UpdateModelConfig(c *gin.Context) {
	adminID, ok := currentAdminID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "not logged in")
		return
	}
	var req dto.AdminModelConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	item, err := adminservice.UpdateModelConfig(c.Request.Context(), c.Param("role"), req, adminID)
	if err != nil {
		writeAdminErr(c, err, "update model config failed")
		return
	}
	response.OK(c, item)
}

func TestModelConfig(c *gin.Context) {
	res, err := adminservice.TestModelConfig(c.Request.Context(), c.Param("role"))
	if err != nil {
		writeAdminErr(c, err, "test model config failed")
		return
	}
	response.OK(c, res)
}

func RestoreModelConfig(c *gin.Context) {
	item, err := adminservice.RestoreModelConfig(c.Request.Context(), c.Param("role"))
	if err != nil {
		writeAdminErr(c, err, "restore model config failed")
		return
	}
	response.OK(c, item)
}

func ApplyModelConfigs(c *gin.Context) {
	res, err := adminservice.ApplyModelConfigs(c.Request.Context())
	if err != nil {
		writeAdminErr(c, err, "apply model configs failed")
		return
	}
	response.OK(c, res)
}

func currentAdminID(c *gin.Context) (uint, bool) {
	v, ok := c.Get(middleware.AdminIDKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(uint)
	return id, ok && id > 0
}

func parseInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

func writeAdminErr(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, adminservice.ErrRegistrationCodeNotConfigured):
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, adminservice.ErrRegistrationCodeInvalid),
		errors.Is(err, adminservice.ErrAdminExists),
		errors.Is(err, errs.ErrCodeExpired),
		errors.Is(err, errs.ErrCodeMismatch):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, adminservice.ErrAdminNotFound),
		errors.Is(err, adminservice.ErrWrongPassword),
		errors.Is(err, adminservice.ErrAdminInactive):
		response.Fail(c, http.StatusUnauthorized, err.Error())
	case errors.Is(err, adminservice.ErrModelRoleInvalid),
		errors.Is(err, adminservice.ErrModelConfigInvalid):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, adminservice.ErrModelRuntimeNotReady):
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, adminservice.ErrModelConfigConnectionFail):
		response.Fail(c, http.StatusBadGateway, err.Error())
	default:
		zlog.Error("admin api failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, fallback)
	}
}
