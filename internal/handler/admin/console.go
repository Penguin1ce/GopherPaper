package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/dto"
	"GopherPaper/internal/middleware"
	"GopherPaper/internal/response"
	adminservice "GopherPaper/internal/service/admin"
	"GopherPaper/internal/zlog"
)

// ── 系统设置 ─────────────────────────────────────────────────

func ListSettings(c *gin.Context) {
	res, err := adminservice.ListSettings(c.Request.Context())
	if err != nil {
		zlog.Error("admin list settings failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

func UpdateSettings(c *gin.Context) {
	var req dto.AdminSettingsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	if err := adminservice.UpdateSettings(c.Request.Context(), req); err != nil {
		zlog.Error("admin update settings failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "update failed")
		return
	}
	response.OKMsg(c, "设置已保存", nil)
}

// ── 站内公告 ─────────────────────────────────────────────────

func ListAnnouncements(c *gin.Context) {
	page := parseInt(c.Query("page"), 1)
	pageSize := parseInt(c.Query("page_size"), 10)
	res, err := adminservice.ListAnnouncements(c.Request.Context(), page, pageSize)
	if err != nil {
		zlog.Error("admin list announcements failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

func CreateAnnouncement(c *gin.Context) {
	var req dto.AdminAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	adminID := c.GetUint(middleware.AdminIDKey)
	adminName := c.GetString(middleware.AdminUsernameKey)
	res, err := adminservice.CreateAnnouncement(c.Request.Context(), req, adminID, adminName)
	if err != nil {
		zlog.Error("admin create announcement failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "create failed")
		return
	}
	response.OK(c, res)
}

func UpdateAnnouncement(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "非法公告 ID")
		return
	}
	var req dto.AdminAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	res, err := adminservice.UpdateAnnouncement(c.Request.Context(), uint(id), req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, res)
}

func DeleteAnnouncement(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "非法公告 ID")
		return
	}
	if err := adminservice.DeleteAnnouncement(c.Request.Context(), uint(id)); err != nil {
		zlog.Error("admin delete announcement failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "delete failed")
		return
	}
	response.OKMsg(c, "公告已删除", nil)
}

// ── 用户反馈 ─────────────────────────────────────────────────

func ListFeedbacks(c *gin.Context) {
	page := parseInt(c.Query("page"), 1)
	pageSize := parseInt(c.Query("page_size"), 10)
	res, err := adminservice.ListFeedbacks(c.Request.Context(), c.Query("status"), page, pageSize)
	if err != nil {
		zlog.Error("admin list feedbacks failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

func UpdateFeedback(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "非法反馈 ID")
		return
	}
	var req dto.AdminFeedbackUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	handlerName := c.GetString(middleware.AdminUsernameKey)
	if err := adminservice.UpdateFeedback(c.Request.Context(), uint(id), req, handlerName); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OKMsg(c, "反馈已更新", nil)
}

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
