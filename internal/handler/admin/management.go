package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/dto"
	"GopherPaper/internal/response"
	adminservice "GopherPaper/internal/service/admin"
	"GopherPaper/internal/zlog"
)

// ── 标签管理 ─────────────────────────────────────────────────

func ListTags(c *gin.Context) {
	res, err := adminservice.ListTags(c.Request.Context())
	if err != nil {
		zlog.Error("admin list tags failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

func RenameTag(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "非法标签 ID")
		return
	}
	var req dto.AdminTagRenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	if err := adminservice.RenameTag(c.Request.Context(), id, req.Name); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OKMsg(c, "标签已重命名", nil)
}

func DeleteTag(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "非法标签 ID")
		return
	}
	if err := adminservice.DeleteTag(c.Request.Context(), id); err != nil {
		zlog.Error("admin delete tag failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "delete failed")
		return
	}
	response.OKMsg(c, "标签已删除", nil)
}

func MergeTags(c *gin.Context) {
	source, err1 := strconv.ParseUint(c.Query("source"), 10, 64)
	target, err2 := strconv.ParseUint(c.Query("target"), 10, 64)
	if err1 != nil || err2 != nil {
		response.Fail(c, http.StatusBadRequest, "非法标签 ID")
		return
	}
	if err := adminservice.MergeTags(c.Request.Context(), source, target); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OKMsg(c, "标签已合并", nil)
}

// ── 会话管理 ─────────────────────────────────────────────────

func ListSessions(c *gin.Context) {
	page := parseInt(c.Query("page"), 1)
	pageSize := parseInt(c.Query("page_size"), 15)
	res, err := adminservice.ListSessions(c.Request.Context(), c.Query("agent_type"), page, pageSize)
	if err != nil {
		zlog.Error("admin list sessions failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

func DeleteSession(c *gin.Context) {
	if err := adminservice.DeleteSession(c.Request.Context(), c.Param("id")); err != nil {
		zlog.Error("admin delete session failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "delete failed")
		return
	}
	response.OKMsg(c, "会话已删除", nil)
}

// ── 管理员账号管理 ───────────────────────────────────────────

func ListAdmins(c *gin.Context) {
	res, err := adminservice.ListAdmins(c.Request.Context())
	if err != nil {
		zlog.Error("admin list admins failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

func SetAdminStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "非法管理员 ID")
		return
	}
	var req dto.AdminAccountStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	if err := adminservice.SetAdminStatus(c.Request.Context(), uint(id), req.Status); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OKMsg(c, "管理员状态已更新", nil)
}

// ── 论文批量操作 ─────────────────────────────────────────────

func BatchDeletePapers(c *gin.Context) {
	var req dto.AdminBatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	res := adminservice.BatchDeletePapers(c.Request.Context(), req.IDs)
	response.OK(c, res)
}
