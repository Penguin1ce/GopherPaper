package admin

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/response"
	adminservice "GopherPaper/internal/service/admin"
	"GopherPaper/internal/zlog"
)

// writeCSV 把表头与数据行以 CSV 写回响应,带 UTF-8 BOM 便于 Excel 正确识别中文。
func writeCSV(c *gin.Context, filename string, header []string, records [][]string) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(c.Writer)
	_ = w.Write(header)
	for _, r := range records {
		_ = w.Write(r)
	}
	w.Flush()
}

// ExportPapersCSV 导出论文列表为 CSV。
func ExportPapersCSV(c *gin.Context) {
	rows, err := adminservice.ExportPapers(c.Request.Context())
	if err != nil {
		zlog.Error("admin export papers failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "export failed")
		return
	}
	records := make([][]string, 0, len(rows))
	for _, p := range rows {
		records = append(records, []string{
			p.ID,
			p.OwnerID,
			p.Title,
			p.FileName,
			string(p.Status),
			strconv.FormatInt(p.Size, 10),
			strconv.Itoa(p.PageCount),
			p.CreatedAt.Format(time.RFC3339),
		})
	}
	writeCSV(c, "papers.csv",
		[]string{"ID", "OwnerID", "Title", "FileName", "Status", "Size", "PageCount", "CreatedAt"},
		records)
}

// ExportUsersCSV 导出用户列表为 CSV。
func ExportUsersCSV(c *gin.Context) {
	rows, err := adminservice.ExportUsers(c.Request.Context())
	if err != nil {
		zlog.Error("admin export users failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "export failed")
		return
	}
	records := make([][]string, 0, len(rows))
	for _, u := range rows {
		records = append(records, []string{
			strconv.FormatUint(uint64(u.ID), 10),
			u.StudentID,
			u.Name,
			u.Email,
			u.ClassID,
			u.CreatedAt.Format(time.RFC3339),
		})
	}
	writeCSV(c, "users.csv",
		[]string{"ID", "StudentID", "Name", "Email", "ClassID", "CreatedAt"},
		records)
}

// ExportLogsCSV 导出服务调用日志为 CSV。
func ExportLogsCSV(c *gin.Context) {
	rows, err := adminservice.ExportLogs(c.Request.Context())
	if err != nil {
		zlog.Error("admin export logs failed", "err", err)
		response.Fail(c, http.StatusInternalServerError, "export failed")
		return
	}
	records := make([][]string, 0, len(rows))
	for _, l := range rows {
		success := "0"
		if l.Success {
			success = "1"
		}
		records = append(records, []string{
			strconv.FormatUint(l.ID, 10),
			l.ServiceType,
			l.ActorID,
			success,
			strconv.FormatInt(l.DurationMS, 10),
			l.ErrorMessage,
			l.CreatedAt.Format(time.RFC3339),
		})
	}
	writeCSV(c, "service_logs.csv",
		[]string{"ID", "ServiceType", "ActorID", "Success", "DurationMS", "ErrorMessage", "CreatedAt"},
		records)
}
