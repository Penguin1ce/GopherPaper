// Package paper 处理论文上传、管理、检索与研读报告的 HTTP 接口。
// 处理函数为裸包级 func,业务委托 service,本层只做参数绑定、身份取用与错误映射。
package paper

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/auth"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/response"
	paperservice "GopherPaper/internal/service/paper"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

// maxUploadBytes 限制上传体积,防止超大文件打满磁盘。
const maxUploadBytes = 50 << 20 // 50MB

// Upload 上传 PDF,建论文并触发异步解析。
// POST /api/v1/papers
func Upload(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "缺少上传文件 file")
		return
	}
	if fileHeader.Size > maxUploadBytes {
		response.Fail(c, http.StatusRequestEntityTooLarge, "文件过大,上限 50MB")
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "读取文件失败")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "读取文件失败")
		return
	}

	p, err := paperservice.Upload(c.Request.Context(), ownerID, fileHeader.Filename, data)
	if err != nil {
		if errors.Is(err, errs.ErrInvalidFile) {
			response.Fail(c, http.StatusBadRequest, "仅支持 PDF 文件")
			return
		}
		zlog.Error("上传论文失败", "owner", ownerID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "上传失败")
		return
	}
	response.OK(c, p)
}

// List 列出我的论文。
// GET /api/v1/papers
func List(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	papers, err := paperservice.List(c.Request.Context(), ownerID)
	if err != nil {
		zlog.Error("查询论文列表失败", "owner", ownerID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	response.OK(c, papers)
}

// Search 历史文献检索。
// GET /api/v1/papers/search?q=xxx
func Search(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	papers, err := paperservice.Search(c.Request.Context(), ownerID, c.Query("q"))
	if err != nil {
		zlog.Error("检索论文失败", "owner", ownerID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "检索失败")
		return
	}
	response.OK(c, papers)
}

// Status 查论文解析状态,SSE 断线兜底用。
// GET /api/v1/papers/:id/status
func Status(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	p, err := paperservice.GetStatus(c.Request.Context(), ownerID, c.Param("id"))
	if err != nil {
		writePaperErr(c, err, "查询失败")
		return
	}
	response.OK(c, gin.H{"id": p.ID, "status": p.Status, "fail_reason": p.FailReason})
}

// Detail 取论文及其结构化元信息与章节。
// GET /api/v1/papers/:id
func Detail(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	p, meta, sections, err := paperservice.Detail(c.Request.Context(), ownerID, c.Param("id"))
	if err != nil {
		writePaperErr(c, err, "查询失败")
		return
	}
	response.OK(c, gin.H{"paper": p, "meta": meta, "sections": sections})
}

// Delete 删除当前用户拥有的论文及其派生数据。
// DELETE /api/v1/papers/:id
func Delete(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	if err := paperservice.Delete(c.Request.Context(), ownerID, c.Param("id")); err != nil {
		writePaperErr(c, err, "删除失败")
		return
	}
	response.OK(c, nil)
}

// Figure 返回某篇论文的某张图片文件,供前端渲染召回引用的缩略图。
// 浏览器 img 标签带不了 Authorization 头,鉴权走 query token,过同一套 auth.Parse。
// GET /api/v1/papers/:id/figures/:name?token=<jwt>
func Figure(c *gin.Context) {
	claims, err := auth.Parse(c.Query("token"))
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "token 无效")
		return
	}
	ownerID := claims.StudentID
	paperID := c.Param("id")
	name := filepath.Base(c.Param("name")) // 防路径穿越,只取文件名
	if name == "." || name == ".." || name == "/" {
		response.Fail(c, http.StatusBadRequest, "非法文件名")
		return
	}
	// 校验论文归属,非本人不得取图。
	if _, err := paperservice.GetStatus(c.Request.Context(), ownerID, paperID); err != nil {
		writePaperErr(c, err, "查询失败")
		return
	}
	path := paperservice.FigurePath(paperID, name)
	if _, err := os.Stat(path); err != nil {
		response.Fail(c, http.StatusNotFound, "图片不存在")
		return
	}
	c.File(path)
}

// File 返回某篇论文的原始 PDF 文件,供精读页 pdf.js 渲染。
// 浏览器/pdf.js 带不了 Authorization 头,鉴权走 query token,过同一套 auth.Parse。
// GET /api/v1/papers/:id/file?token=<jwt>
func File(c *gin.Context) {
	claims, err := auth.Parse(c.Query("token"))
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "token 无效")
		return
	}
	path, err := paperservice.PaperFile(c.Request.Context(), claims.StudentID, c.Param("id"))
	if err != nil {
		writePaperErr(c, err, "查询失败")
		return
	}
	if _, err := os.Stat(path); err != nil {
		response.Fail(c, http.StatusNotFound, "文件不存在")
		return
	}
	c.Header("Content-Type", "application/pdf")
	c.File(path)
}

// Report 按报告类型生成研读报告,前端按钮触发。
// POST /api/v1/papers/:id/report
func Report(c *gin.Context) {
	var req dto.ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	reportType := constant.ReportType(req.Type)
	if !reportType.Valid() {
		response.Fail(c, http.StatusBadRequest, "不支持的报告类型")
		return
	}
	ownerID := tenant.MustStudentID(c.Request.Context())
	paperID := c.Param("id")
	// 校验归属 + 命中缓存复用,未命中才生成并落库。
	reply, err := paperservice.Report(c.Request.Context(), ownerID, paperID, reportType)
	if err != nil {
		if errors.Is(err, errs.ErrPaperNotFound) || errors.Is(err, errs.ErrPaperForbidden) {
			writePaperErr(c, err, "生成失败")
			return
		}
		if errors.Is(err, errs.ErrReportGenerating) {
			// 后台正在预生成,前端稍后重试即可命中缓存。
			response.Fail(c, http.StatusAccepted, "报告正在生成中，请稍候重试")
			return
		}
		zlog.Error("生成研读报告失败", "paper_id", paperID, "type", req.Type, "err", err)
		response.Fail(c, http.StatusInternalServerError, "生成失败")
		return
	}
	response.OK(c, dto.ChatResponse{Intent: string(reply.Intent), Content: reply.Content, Meta: reply.Meta})
}

// Reports 列出某篇论文已生成的研读报告类型,前端进入论文时回填就绪态并自动展示,不触发生成。
// GET /api/v1/papers/:id/reports
func Reports(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	types, running, err := paperservice.ReportOverview(c.Request.Context(), ownerID, c.Param("id"))
	if err != nil {
		writePaperErr(c, err, "查询失败")
		return
	}
	if types == nil {
		types = []constant.ReportType{}
	}
	if running == nil {
		running = []paperservice.ReportRun{}
	}
	response.OK(c, gin.H{"ready": types, "running": running})
}

// Translate 把精读页选中的英文原文译成中文,前端选区触发,不经分类器、不走 RAG。
// POST /api/v1/papers/:id/translate
func Translate(c *gin.Context) {
	var req dto.TranslateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		response.Fail(c, http.StatusBadRequest, "原文为空")
		return
	}
	if utf8.RuneCountInString(text) > constant.MaxTranslateRunes {
		response.Fail(c, http.StatusBadRequest, "选段过长,请缩短后重试")
		return
	}
	ownerID := tenant.MustStudentID(c.Request.Context())
	// 校验论文归属,确保 :id 属于本人(翻译按 owner 选其小模型)。
	if _, err := paperservice.GetStatus(c.Request.Context(), ownerID, c.Param("id")); err != nil {
		writePaperErr(c, err, "查询失败")
		return
	}
	translation, err := ai.Translate(c.Request.Context(), text)
	if err != nil {
		zlog.Error("翻译失败", "owner", ownerID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "翻译失败")
		return
	}
	response.OK(c, gin.H{"translation": translation})
}

// writePaperErr 把论文错误映射为对应 HTTP 状态。
func writePaperErr(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, errs.ErrPaperNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, errs.ErrPaperForbidden):
		response.Fail(c, http.StatusForbidden, err.Error())
	default:
		zlog.Error("论文接口错误", "err", err)
		response.Fail(c, http.StatusInternalServerError, fallback)
	}
}
