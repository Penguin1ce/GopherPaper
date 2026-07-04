package user

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/dto"
	"GopherPaper/internal/response"
	adminservice "GopherPaper/internal/service/admin"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
)

func ListModelConfigs(c *gin.Context) {
	studentID, ok := currentStudentID(c)
	if !ok {
		return
	}
	res, err := adminservice.ListUserModelConfigs(c.Request.Context(), studentID)
	if err != nil {
		writeModelConfigErr(c, err, "模型配置加载失败")
		return
	}
	response.OK(c, res)
}

func UpdateModelConfig(c *gin.Context) {
	studentID, ok := currentStudentID(c)
	if !ok {
		return
	}
	var req dto.AdminModelConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	item, err := adminservice.UpdateUserModelConfig(c.Request.Context(), studentID, c.Param("role"), req)
	if err != nil {
		writeModelConfigErr(c, err, "模型配置保存失败")
		return
	}
	response.OK(c, item)
}

func TestModelConfig(c *gin.Context) {
	studentID, ok := currentStudentID(c)
	if !ok {
		return
	}
	res, err := adminservice.TestUserModelConfig(c.Request.Context(), studentID, c.Param("role"))
	if err != nil {
		writeModelConfigErr(c, err, "模型连通测试失败")
		return
	}
	response.OK(c, res)
}

func RestoreModelConfig(c *gin.Context) {
	studentID, ok := currentStudentID(c)
	if !ok {
		return
	}
	item, err := adminservice.RestoreUserModelConfig(c.Request.Context(), studentID, c.Param("role"))
	if err != nil {
		writeModelConfigErr(c, err, "恢复模型配置失败")
		return
	}
	response.OK(c, item)
}

func ApplyModelConfigs(c *gin.Context) {
	studentID, ok := currentStudentID(c)
	if !ok {
		return
	}
	res, err := adminservice.ApplyUserModelConfigs(c.Request.Context(), studentID)
	if err != nil {
		writeModelConfigErr(c, err, "应用模型配置失败")
		return
	}
	response.OK(c, res)
}

func currentStudentID(c *gin.Context) (string, bool) {
	studentID := tenant.MustStudentID(c.Request.Context())
	if studentID == "" {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return "", false
	}
	return studentID, true
}

func writeModelConfigErr(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, adminservice.ErrModelRoleInvalid),
		errors.Is(err, adminservice.ErrModelConfigInvalid):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, adminservice.ErrModelRuntimeNotReady):
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, adminservice.ErrModelConfigConnectionFail):
		response.Fail(c, http.StatusBadGateway, err.Error())
	default:
		zlog.Error("user model config api failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, fallback)
	}
}
