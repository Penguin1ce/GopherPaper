package admin

import (
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/auth"
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
	auth.SetAdminCookie(c.Writer, c.Request, token)
	response.OK(c, dto.AdminLoginResponse{Admin: adminservice.Profile(admin)})
}

func Logout(c *gin.Context) {
	adminID, ok := currentAdminID(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, "not logged in")
		return
	}
	if err := adminservice.Logout(c.Request.Context(), adminID); err != nil {
		zlog.Error("admin logout failed", "admin_id", adminID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "logout failed")
		return
	}
	auth.ClearAdminCookie(c.Writer, c.Request)
	response.OKMsg(c, "logged out", nil)
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

// Analytics 返回看板所需的时间序列、状态分布与服务统计。
func Analytics(c *gin.Context) {
	days := parseInt(c.Query("days"), 30)
	data, err := adminservice.Analytics(c.Request.Context(), days)
	if err != nil {
		zlog.Error("admin analytics failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, data)
}

// Health 返回后台依赖中间件的实时健康快照。
func Health(c *gin.Context) {
	response.OK(c, adminservice.Health(c.Request.Context()))
}

// Activity 返回最近的运营事件时间线。
func Activity(c *gin.Context) {
	limit := parseInt(c.Query("limit"), 20)
	data, err := adminservice.Activity(c.Request.Context(), limit)
	if err != nil {
		zlog.Error("admin activity failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, data)
}

// SeedDemo 灌入演示数据,便于中期检查演示看板。
func SeedDemo(c *gin.Context) {
	res, err := adminservice.SeedDemo(c.Request.Context())
	if err != nil {
		zlog.Error("admin seed demo failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "seed failed")
		return
	}
	response.OK(c, res)
}

// ClearDemo 清除全部演示数据。
func ClearDemo(c *gin.Context) {
	res, err := adminservice.ClearDemo(c.Request.Context())
	if err != nil {
		zlog.Error("admin clear demo failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "clear failed")
		return
	}
	response.OK(c, res)
}

// ListUsers 分页返回用户列表(支持搜索与班级筛选)。
func ListUsers(c *gin.Context) {
	page := parseInt(c.Query("page"), 1)
	pageSize := parseInt(c.Query("page_size"), 10)
	res, err := adminservice.ListUsers(c.Request.Context(), c.Query("query"), c.Query("class_id"), page, pageSize)
	if err != nil {
		zlog.Error("admin list users failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

// GetUser 返回单个用户画像。
func GetUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "非法用户 ID")
		return
	}
	res, err := adminservice.GetUserDetail(c.Request.Context(), uint(id))
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}
	response.OK(c, res)
}

// ClassStats 返回按班级聚合的统计。
func ClassStats(c *gin.Context) {
	res, err := adminservice.ClassStats(c.Request.Context())
	if err != nil {
		zlog.Error("admin class stats failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

// ListLogs 分页返回服务调用日志(支持筛选)。
func ListLogs(c *gin.Context) {
	page := parseInt(c.Query("page"), 1)
	pageSize := parseInt(c.Query("page_size"), 20)
	res, err := adminservice.ListLogs(c.Request.Context(), c.Query("service_type"), c.Query("result"), c.Query("actor"), page, pageSize)
	if err != nil {
		zlog.Error("admin list logs failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

// LogStats 返回日志筛选辅助统计。
func LogStats(c *gin.Context) {
	res, err := adminservice.LogStats(c.Request.Context())
	if err != nil {
		zlog.Error("admin log stats failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

// AdvancedAnalytics 返回高级分析页所需的多维统计。
func AdvancedAnalytics(c *gin.Context) {
	res, err := adminservice.AdvancedAnalytics(c.Request.Context())
	if err != nil {
		zlog.Error("admin advanced analytics failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
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
	default:
		zlog.Error("admin api failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, fallback)
	}
}
