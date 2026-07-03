package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/dto"
	"GopherPaper/internal/middleware"
	"GopherPaper/internal/response"
	adminservice "GopherPaper/internal/service/admin"
	"GopherPaper/internal/zlog"
)

// taskActor 从上下文取当前管理员的 ID 与用户名,用于活动记录。
func taskActor(c *gin.Context) (uint, string) {
	return c.GetUint(middleware.AdminIDKey), c.GetString(middleware.AdminUsernameKey)
}

// parseTaskID 解析路径上的任务 ID。
func parseTaskID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "非法任务 ID")
		return 0, false
	}
	return uint(id), true
}

// ── 看板 / 列表 / 统计 ────────────────────────────────────────

func TaskBoard(c *gin.Context) {
	res, err := adminservice.BoardTasks(c.Request.Context(), c.Query("assignee"), c.Query("priority"), c.Query("query"))
	if err != nil {
		zlog.Error("admin task board failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

func ListTasks(c *gin.Context) {
	page := parseInt(c.Query("page"), 1)
	pageSize := parseInt(c.Query("page_size"), 20)
	res, err := adminservice.ListTasks(c.Request.Context(),
		c.Query("status"), c.Query("assignee"), c.Query("priority"), c.Query("query"), page, pageSize)
	if err != nil {
		zlog.Error("admin list tasks failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

func TaskStats(c *gin.Context) {
	res, err := adminservice.TaskStats(c.Request.Context())
	if err != nil {
		zlog.Error("admin task stats failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

// ── 任务 CRUD ─────────────────────────────────────────────────

func CreateTask(c *gin.Context) {
	var req dto.AdminTaskCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	actorID, actorName := taskActor(c)
	res, err := adminservice.CreateTask(c.Request.Context(), req, actorID, actorName)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, res)
}

func GetTaskDetail(c *gin.Context) {
	id, ok := parseTaskID(c)
	if !ok {
		return
	}
	res, err := adminservice.GetTaskDetail(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}
	response.OK(c, res)
}

func UpdateTask(c *gin.Context) {
	id, ok := parseTaskID(c)
	if !ok {
		return
	}
	var req dto.AdminTaskUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	actorID, actorName := taskActor(c)
	res, err := adminservice.UpdateTask(c.Request.Context(), id, req, actorID, actorName)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, res)
}

func MoveTask(c *gin.Context) {
	id, ok := parseTaskID(c)
	if !ok {
		return
	}
	var req dto.AdminTaskMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	actorID, actorName := taskActor(c)
	res, err := adminservice.MoveTask(c.Request.Context(), id, req, actorID, actorName)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, res)
}

func DeleteTask(c *gin.Context) {
	id, ok := parseTaskID(c)
	if !ok {
		return
	}
	if err := adminservice.DeleteTask(c.Request.Context(), id); err != nil {
		zlog.Error("admin delete task failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "delete failed")
		return
	}
	response.OKMsg(c, "任务已删除", nil)
}

// ── 批量 / 导出 ───────────────────────────────────────────────

func BulkTasks(c *gin.Context) {
	var req dto.AdminTaskBulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	actorID, actorName := taskActor(c)
	res, err := adminservice.BulkTasks(c.Request.Context(), req, actorID, actorName)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, res)
}

func ExportTasksCSV(c *gin.Context) {
	rows, err := adminservice.ExportTasks(c.Request.Context())
	if err != nil {
		zlog.Error("admin export tasks failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "export failed")
		return
	}
	records := make([][]string, 0, len(rows))
	for _, t := range rows {
		due := ""
		if t.DueAt != nil {
			due = t.DueAt.Format(time.RFC3339)
		}
		records = append(records, []string{
			strconv.FormatUint(uint64(t.ID), 10),
			t.Title,
			t.Status,
			t.Priority,
			t.Assignee,
			due,
			t.CreatorName,
			t.CreatedAt.Format(time.RFC3339),
		})
	}
	writeCSV(c, "tasks.csv",
		[]string{"ID", "Title", "Status", "Priority", "Assignee", "DueAt", "Creator", "CreatedAt"},
		records)
}

// ── 评论 ─────────────────────────────────────────────────────

func ListTaskComments(c *gin.Context) {
	id, ok := parseTaskID(c)
	if !ok {
		return
	}
	res, err := adminservice.ListTaskComments(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

func AddTaskComment(c *gin.Context) {
	id, ok := parseTaskID(c)
	if !ok {
		return
	}
	var req dto.AdminTaskCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	actorID, actorName := taskActor(c)
	res, err := adminservice.AddTaskComment(c.Request.Context(), id, req.Content, actorID, actorName)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, res)
}

func DeleteTaskComment(c *gin.Context) {
	id, ok := parseTaskID(c)
	if !ok {
		return
	}
	commentID, err := strconv.ParseUint(c.Param("commentId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "非法评论 ID")
		return
	}
	if err := adminservice.DeleteTaskComment(c.Request.Context(), id, uint(commentID)); err != nil {
		response.Fail(c, http.StatusInternalServerError, "delete failed")
		return
	}
	response.OKMsg(c, "评论已删除", nil)
}

// ── 子清单 ───────────────────────────────────────────────────

func ListChecklist(c *gin.Context) {
	id, ok := parseTaskID(c)
	if !ok {
		return
	}
	res, err := adminservice.ListChecklist(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

func AddChecklistItem(c *gin.Context) {
	id, ok := parseTaskID(c)
	if !ok {
		return
	}
	var req dto.AdminTaskChecklistCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	actorID, actorName := taskActor(c)
	res, err := adminservice.AddChecklistItem(c.Request.Context(), id, req.Content, actorID, actorName)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, res)
}

func UpdateChecklistItem(c *gin.Context) {
	id, ok := parseTaskID(c)
	if !ok {
		return
	}
	itemID, err := strconv.ParseUint(c.Param("itemId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "非法子任务 ID")
		return
	}
	var req dto.AdminTaskChecklistUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	actorID, actorName := taskActor(c)
	res, err := adminservice.UpdateChecklistItem(c.Request.Context(), id, uint(itemID), req, actorID, actorName)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, res)
}

func DeleteChecklistItem(c *gin.Context) {
	id, ok := parseTaskID(c)
	if !ok {
		return
	}
	itemID, err := strconv.ParseUint(c.Param("itemId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "非法子任务 ID")
		return
	}
	res, err := adminservice.DeleteChecklistItem(c.Request.Context(), id, uint(itemID))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "delete failed")
		return
	}
	response.OK(c, res)
}

// ── 活动时间线 ───────────────────────────────────────────────

func ListTaskActivities(c *gin.Context) {
	id, ok := parseTaskID(c)
	if !ok {
		return
	}
	res, err := adminservice.ListTaskActivities(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "query failed")
		return
	}
	response.OK(c, res)
}

// ── 演示数据 ─────────────────────────────────────────────────

func SeedDemoTasks(c *gin.Context) {
	actorID, actorName := taskActor(c)
	res, err := adminservice.SeedDemoTasks(c.Request.Context(), actorID, actorName)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, res)
}

func ClearDemoTasks(c *gin.Context) {
	res, err := adminservice.ClearDemoTasks(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, res)
}
