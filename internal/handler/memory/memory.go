// Package memory 处理个人长期记忆卡片接口。
package memory

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/dto"
	"GopherPaper/internal/response"
	memoryservice "GopherPaper/internal/service/memory"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/errs"
)

func List(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	req := dto.MemorySearchRequest{
		Q:    c.Query("q"),
		Type: c.Query("type"),
		Tag:  c.Query("tag"),
	}
	items, err := memoryservice.List(c.Request.Context(), studentID, req)
	if err != nil {
		zlog.Error("查询记忆失败", "student_id", studentID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询记忆失败")
		return
	}
	response.OK(c, items)
}

func Search(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	var req dto.MemorySearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	items, err := memoryservice.List(c.Request.Context(), studentID, req)
	if err != nil {
		zlog.Error("搜索记忆失败", "student_id", studentID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "搜索记忆失败")
		return
	}
	response.OK(c, items)
}

func Create(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	var req dto.MemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	item, err := memoryservice.Create(c.Request.Context(), studentID, req)
	if err != nil {
		writeMemoryErr(c, err)
		return
	}
	response.OK(c, item)
}

func Update(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	var req dto.MemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	item, err := memoryservice.Update(c.Request.Context(), studentID, c.Param("id"), req)
	if err != nil {
		writeMemoryErr(c, err)
		return
	}
	response.OK(c, item)
}

func Delete(c *gin.Context) {
	studentID := tenant.MustStudentID(c.Request.Context())
	if err := memoryservice.Delete(c.Request.Context(), studentID, c.Param("id")); err != nil {
		writeMemoryErr(c, err)
		return
	}
	response.OK(c, nil)
}

func writeMemoryErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errs.ErrMemoryInvalid):
		response.Fail(c, http.StatusBadRequest, "记忆标题或内容不合法")
	case errors.Is(err, errs.ErrMemoryNotFound):
		response.Fail(c, http.StatusNotFound, "记忆不存在")
	default:
		zlog.Error("记忆接口错误", "err", err)
		response.Fail(c, http.StatusInternalServerError, "记忆操作失败")
	}
}
