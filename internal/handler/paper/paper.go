// Package paper 处理论文上传、管理、检索与研读报告的 HTTP 接口。
// 处理函数为裸包级 func,业务委托 service,本层只做参数绑定、身份取用与错误映射。
package paper

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/auth"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
	"GopherPaper/internal/response"
	paperservice "GopherPaper/internal/service/paper"
	"GopherPaper/internal/sse"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
	"GopherPaper/pkg/errs"
)

// maxUploadBytes 限制上传体积,防止超大文件打满磁盘。
const maxUploadBytes = 50 << 20 // 50MB

// Upload 上传 PDF,建论文并触发异步解析。
// POST /api/v1/papers
//
// @Summary 上传论文
// @Description 上传 PDF 文件，创建论文记录并投递异步解析任务。文件大小上限 50MB。
// @Tags papers
// @Accept mpfd
// @Produce json
// @Security BearerAuth
// @Param file formData file true "PDF 文件"
// @Success 200 {object} dto.Response{data=model.Paper}
// @Failure 400 {object} dto.Response
// @Failure 413 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers [post]
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
//
// @Summary 列出我的论文
// @Description 返回当前登录用户上传或导入的论文列表。
// @Tags papers
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.Response{data=[]model.Paper}
// @Failure 500 {object} dto.Response
// @Router /papers [get]
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
//
// @Summary 搜索论文库
// @Description 在当前用户论文库中按关键词检索历史文献。
// @Tags papers
// @Produce json
// @Security BearerAuth
// @Param q query string false "搜索关键词"
// @Success 200 {object} dto.Response{data=[]model.Paper}
// @Failure 500 {object} dto.Response
// @Router /papers/search [get]
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

// Compare 多论文对比分析。
// POST /api/v1/papers/compare
func Compare(c *gin.Context) {
	var req dto.PaperCompareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	ids := normalizeRequestPaperIDs(req.PaperIDs)
	if len(ids) < 2 {
		response.Fail(c, http.StatusBadRequest, "请至少选择两篇论文")
		return
	}
	if len(ids) > constant.ComparePapersMaxCount {
		response.Fail(c, http.StatusBadRequest, "单次最多对比 "+strconv.Itoa(constant.ComparePapersMaxCount)+" 篇论文")
		return
	}
	ownerID := tenant.MustStudentID(c.Request.Context())
	report, err := paperservice.Compare(c.Request.Context(), ownerID, ids)
	if err != nil {
		if errors.Is(err, errs.ErrPaperNotFound) || errors.Is(err, errs.ErrPaperForbidden) {
			writePaperErr(c, err, "对比失败")
			return
		}
		zlog.Error("多论文对比失败", "owner", ownerID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "对比失败")
		return
	}
	response.OK(c, report)
}

// CompareReports 列出当前用户的历史多论文对比报告。
// GET /api/v1/papers/compare/reports
func CompareReports(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	reports, err := paperservice.ListCompareReports(c.Request.Context(), ownerID)
	if err != nil {
		zlog.Error("查询多论文对比报告失败", "owner", ownerID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询对比报告失败")
		return
	}
	response.OK(c, reports)
}

// DeleteCompareReport 删除当前用户的一份历史多论文对比报告。
// DELETE /api/v1/papers/compare/reports/:report_id
func DeleteCompareReport(c *gin.Context) {
	reportID, ok := parseCompareReportID(c)
	if !ok {
		return
	}
	ownerID := tenant.MustStudentID(c.Request.Context())
	if err := paperservice.DeleteCompareReport(c.Request.Context(), ownerID, reportID); err != nil {
		writePaperErr(c, err, "删除对比报告失败")
		return
	}
	response.OK(c, nil)
}

// Status 查论文解析状态,SSE 断线兜底用。
// GET /api/v1/papers/:id/status
//
// @Summary 查询论文解析状态
// @Description 返回论文解析状态，供 SSE 断线后的兜底轮询使用。
// @Tags papers
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Success 200 {object} dto.Response{data=dto.PaperStatusResponse}
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id}/status [get]
func Status(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	p, err := paperservice.GetStatus(c.Request.Context(), ownerID, c.Param("id"))
	if err != nil {
		writePaperErr(c, err, "查询失败")
		return
	}
	response.OK(c, gin.H{
		"id": p.ID, "status": p.Status, "fail_reason": p.FailReason,
		"parse_progress": p.ParseProgress, "parsed_pages": p.ParsedPages, "total_pages": p.TotalPages,
	})
}

// Reparse 重新投递当前用户拥有的论文解析任务。
// POST /api/v1/papers/:id/reparse
//
// @Summary 重新解析论文
// @Description 清理旧报告缓存并重新投递论文解析任务。正在解析中的论文不可重复投递。
// @Tags papers
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Success 200 {object} dto.Response{data=model.Paper}
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 409 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id}/reparse [post]
func Reparse(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	p, err := paperservice.Reparse(c.Request.Context(), ownerID, c.Param("id"))
	if err != nil {
		writePaperErr(c, err, "重新解析失败")
		return
	}
	response.OK(c, p)
}

// Detail 取论文及其结构化元信息与章节。
// GET /api/v1/papers/:id
//
// @Summary 获取论文详情
// @Description 返回论文基础信息、结构化元信息与章节目录。
// @Tags papers
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Success 200 {object} dto.Response{data=dto.PaperDetailResponse}
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id} [get]
func Detail(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	p, meta, sections, err := paperservice.Detail(c.Request.Context(), ownerID, c.Param("id"))
	if err != nil {
		writePaperErr(c, err, "查询失败")
		return
	}
	response.OK(c, gin.H{"paper": p, "meta": meta, "sections": sections})
}

// RebuildSections 从已归档的 MinerU 产物重建论文目录。
// POST /api/v1/papers/:id/sections/rebuild
//
// @Summary 重建论文目录
// @Description 从本地归档的 MinerU content_list.json 重建章节目录，不重跑完整解析流水线。
// @Tags papers
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Success 200 {object} dto.Response{data=[]model.PaperSection}
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id}/sections/rebuild [post]
func RebuildSections(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	sections, err := paperservice.RebuildSections(c.Request.Context(), ownerID, c.Param("id"))
	if err != nil {
		writePaperErr(c, err, "重建目录失败")
		return
	}
	response.OK(c, sections)
}

// Delete 删除当前用户拥有的论文及其派生数据。
// DELETE /api/v1/papers/:id
//
// @Summary 删除论文
// @Description 删除当前用户拥有的论文及其派生数据，包括会话、报告、图片、向量索引与知识图谱节点。
// @Tags papers
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Success 200 {object} dto.Response
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id} [delete]
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
//
// @Summary 读取论文图片
// @Description 返回某篇论文解析出的图片文件。该接口供浏览器图片标签使用，鉴权使用 query token。
// @Tags papers
// @Produce octet-stream
// @Param id path string true "论文 ID"
// @Param name path string true "图片文件名"
// @Param token query string true "JWT"
// @Success 200 {file} file
// @Failure 400 {object} dto.Response
// @Failure 401 {object} dto.Response
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id}/figures/{name} [get]
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
//
// @Summary 读取论文 PDF
// @Description 返回某篇论文的原始 PDF 文件。该接口供 pdf.js 使用，鉴权使用 query token。
// @Tags papers
// @Produce octet-stream
// @Param id path string true "论文 ID"
// @Param token query string true "JWT"
// @Success 200 {file} file
// @Failure 401 {object} dto.Response
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id}/file [get]
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
//
// @Summary 生成研读报告
// @Description 按报告类型生成或读取缓存的研读报告。报告类型包括 quickread、method、result、innovation、related。
// @Tags papers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Param request body dto.ReportRequest true "报告类型"
// @Success 200 {object} dto.Response{data=dto.ChatResponse}
// @Success 202 {object} dto.Response "报告正在生成中"
// @Failure 400 {object} dto.Response
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id}/report [post]
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

// Flow 为某篇论文生成小云雀同款研究思路图,返回节点图 JSON,前端按钮触发。
// POST /api/v1/papers/:id/flow
//
// @Summary 生成论文思路图
// @Description 复用小云雀同款思路图链路生成研究脉络,返回可持久化的节点图 JSON。
// @Tags papers
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Success 200 {object} dto.Response{data=dto.ChatResponse}
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 409 {object} dto.Response "论文尚未解析就绪"
// @Failure 500 {object} dto.Response
// @Router /papers/{id}/flow [post]
func Flow(c *gin.Context) {
	if strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
		flowStream(c)
		return
	}
	ownerID := tenant.MustStudentID(c.Request.Context())
	paperID := c.Param("id")
	reply, err := paperservice.PaperFlow(c.Request.Context(), ownerID, paperID)
	if err != nil {
		if errors.Is(err, errs.ErrPaperNotFound) || errors.Is(err, errs.ErrPaperForbidden) ||
			errors.Is(err, errs.ErrPaperNotReady) {
			writePaperErr(c, err, "生成失败")
			return
		}
		zlog.Error("生成论文思路图失败", "paper_id", paperID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "生成失败")
		return
	}
	response.OK(c, dto.ChatResponse{Intent: string(reply.Intent), Content: reply.Content, Meta: reply.Meta})
}

// GetFlow 只读取某篇论文已生成的思路图缓存,不触发生成。
// GET /api/v1/papers/:id/flow
func GetFlow(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	paperID := c.Param("id")
	reply, err := paperservice.GetPaperFlow(c.Request.Context(), ownerID, paperID)
	if err != nil {
		if errors.Is(err, errs.ErrPaperNotFound) || errors.Is(err, errs.ErrPaperForbidden) ||
			errors.Is(err, errs.ErrPaperFlowNotFound) {
			writePaperErr(c, err, "查询失败")
			return
		}
		zlog.Error("查询论文思路图失败", "paper_id", paperID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询失败")
		return
	}
	response.OK(c, dto.ChatResponse{Intent: string(reply.Intent), Content: reply.Content, Meta: reply.Meta})
}

func flowStream(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	paperID := c.Param("id")

	started := false
	emit := func(name string, payload any) {
		if !started {
			sse.WriteHeaders(c)
			c.Writer.WriteHeader(http.StatusOK)
			started = true
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return
		}
		sse.WriteEvent(c.Writer, name, string(b))
	}

	ctx := core.WithStream(c.Request.Context(), func(ev core.StreamEvent) {
		switch ev.Kind {
		case constant.StreamEventPaperFlow, constant.StreamEventPaperFlowNode:
			emit(ev.Kind, ev.Payload)
		}
	})
	reply, err := paperservice.PaperFlow(ctx, ownerID, paperID)
	if err != nil {
		if !started {
			if errors.Is(err, errs.ErrPaperNotFound) || errors.Is(err, errs.ErrPaperForbidden) ||
				errors.Is(err, errs.ErrPaperNotReady) {
				writePaperErr(c, err, "生成失败")
				return
			}
			zlog.Error("生成论文思路图失败", "paper_id", paperID, "err", err)
			response.Fail(c, http.StatusInternalServerError, "生成失败")
			return
		}
		zlog.Error("流式生成论文思路图失败", "paper_id", paperID, "err", err)
		emit(constant.StreamEventError, dto.StreamErrorPayload{Message: "生成失败"})
		return
	}
	emit(constant.StreamEventDone, dto.ChatResponse{Intent: string(reply.Intent), Content: reply.Content, Meta: reply.Meta})
}

// Reports 列出某篇论文已生成的研读报告类型,前端进入论文时回填就绪态并自动展示,不触发生成。
// GET /api/v1/papers/:id/reports
//
// @Summary 列出已生成报告
// @Description 返回某篇论文已生成并可直接读取的研读报告类型。
// @Tags papers
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Success 200 {object} dto.Response{data=dto.ReadyReportsResponse}
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id}/reports [get]
func Reports(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	types, running, flowReady, err := paperservice.ReportOverview(c.Request.Context(), ownerID, c.Param("id"))
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
	response.OK(c, dto.ReadyReportsResponse{
		Ready:     types,
		Running:   reportRunsToDTO(running),
		FlowReady: flowReady,
	})
}

func reportRunsToDTO(runs []paperservice.ReportRun) []dto.ReportRun {
	out := make([]dto.ReportRun, 0, len(runs))
	for _, run := range runs {
		steps := make([]dto.ReportProgressStep, 0, len(run.Steps))
		for _, step := range run.Steps {
			steps = append(steps, dto.ReportProgressStep{Phase: step.Phase, Text: step.Text})
		}
		out = append(out, dto.ReportRun{
			Type:   run.ReportType,
			Steps:  steps,
			Live:   run.Live,
			Failed: run.Failed,
		})
	}
	return out
}

// Translate 把精读页选中的英文原文译成中文,前端选区触发,不经分类器、不走 RAG。
// POST /api/v1/papers/:id/translate
//
// @Summary 翻译论文选段
// @Description 将精读页选中的英文原文翻译成中文，不经过聊天意图分类与 RAG。
// @Tags papers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Param request body dto.TranslateRequest true "待翻译文本"
// @Success 200 {object} dto.Response{data=dto.TranslateResponse}
// @Failure 400 {object} dto.Response
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id}/translate [post]
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

// UpdateProgress 保存精读页最近阅读页与百分比进度。
// PATCH /api/v1/papers/:id/progress
//
// @Summary 保存阅读进度
// @Description 保存某篇论文的最近阅读页，并按总页数计算阅读百分比。
// @Tags papers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Param request body dto.PaperProgressRequest true "阅读进度"
// @Success 200 {object} dto.Response{data=dto.PaperProgressResponse}
// @Failure 400 {object} dto.Response
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id}/progress [patch]
func UpdateProgress(c *gin.Context) {
	var req dto.PaperProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	ownerID := tenant.MustStudentID(c.Request.Context())
	p, err := paperservice.UpdateReadProgress(c.Request.Context(), ownerID, c.Param("id"), req.LastPage, req.TotalPages)
	if err != nil {
		if writePaperAccessErr(c, err, "保存失败") {
			return
		}
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, dto.PaperProgressResponse{Progress: p.Progress, LastReadPage: p.LastReadPage})
}

// ListAnnotations 列出某篇论文的精读批注。
// GET /api/v1/papers/:id/annotations
//
// @Summary 列出精读批注
// @Description 返回某篇论文当前用户保存的全部高亮与批注。
// @Tags papers
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Success 200 {object} dto.Response{data=[]model.PaperAnnotation}
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id}/annotations [get]
func ListAnnotations(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	annotations, err := paperservice.ListAnnotations(c.Request.Context(), ownerID, c.Param("id"))
	if err != nil {
		writePaperErr(c, err, "查询失败")
		return
	}
	response.OK(c, annotations)
}

// CreateAnnotation 新建一条精读批注。
// POST /api/v1/papers/:id/annotations
//
// @Summary 新建精读批注
// @Description 保存一段 PDF 原文的高亮坐标与可选笔记。
// @Tags papers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Param request body dto.AnnotationCreateRequest true "批注内容"
// @Success 200 {object} dto.Response{data=model.PaperAnnotation}
// @Failure 400 {object} dto.Response
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id}/annotations [post]
func CreateAnnotation(c *gin.Context) {
	var req dto.AnnotationCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	ownerID := tenant.MustStudentID(c.Request.Context())
	annotation, err := paperservice.CreateAnnotation(c.Request.Context(), ownerID, c.Param("id"), paperservice.AnnotationInput{
		PageNo:       req.PageNo,
		Kind:         constant.AnnotationKind(req.Kind),
		Text:         req.Text,
		Note:         req.Note,
		Translation:  req.Translation,
		Color:        req.Color,
		BoundingRect: dtoRectToModel(req.BoundingRect),
		Rects:        dtoRectsToModel(req.Rects),
		StyleJSON:    req.StyleJSON,
		ContentJSON:  req.ContentJSON,
	})
	if err != nil {
		if writePaperAccessErr(c, err, "保存失败") {
			return
		}
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, annotation)
}

// UpdateAnnotation 更新精读批注内容、位置或样式。
// PATCH /api/v1/papers/:id/annotations/:annotation_id
//
// @Summary 更新精读批注
// @Description 更新某条精读批注的内容、位置或样式。
// @Tags papers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Param annotation_id path int true "批注 ID"
// @Param request body dto.AnnotationUpdateRequest true "批注更新内容"
// @Success 200 {object} dto.Response{data=model.PaperAnnotation}
// @Failure 400 {object} dto.Response
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id}/annotations/{annotation_id} [patch]
func UpdateAnnotation(c *gin.Context) {
	annotationID, ok := parseAnnotationID(c)
	if !ok {
		return
	}
	var req dto.AnnotationUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	ownerID := tenant.MustStudentID(c.Request.Context())
	var rects *model.AnnotationRects
	if req.Rects != nil {
		converted := dtoRectsToModel(req.Rects)
		rects = &converted
	}
	var boundingRect *model.AnnotationRect
	if req.BoundingRect != nil {
		converted := dtoRectToModel(*req.BoundingRect)
		boundingRect = &converted
	}
	annotation, err := paperservice.UpdateAnnotation(c.Request.Context(), ownerID, c.Param("id"), annotationID, paperservice.AnnotationUpdateInput{
		Text:         req.Text,
		Note:         req.Note,
		Translation:  req.Translation,
		Color:        req.Color,
		BoundingRect: boundingRect,
		Rects:        rects,
		StyleJSON:    req.StyleJSON,
		ContentJSON:  req.ContentJSON,
	})
	if err != nil {
		if writePaperAccessErr(c, err, "更新失败") {
			return
		}
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, annotation)
}

// DeleteAnnotation 删除一条精读批注。
// DELETE /api/v1/papers/:id/annotations/:annotation_id
//
// @Summary 删除精读批注
// @Description 删除某条高亮或批注。
// @Tags papers
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Param annotation_id path int true "批注 ID"
// @Success 200 {object} dto.Response
// @Failure 400 {object} dto.Response
// @Failure 403 {object} dto.Response
// @Failure 404 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /papers/{id}/annotations/{annotation_id} [delete]
func DeleteAnnotation(c *gin.Context) {
	annotationID, ok := parseAnnotationID(c)
	if !ok {
		return
	}
	ownerID := tenant.MustStudentID(c.Request.Context())
	if err := paperservice.DeleteAnnotation(c.Request.Context(), ownerID, c.Param("id"), annotationID); err != nil {
		writePaperErr(c, err, "删除失败")
		return
	}
	response.OK(c, nil)
}

func BuildMindMap(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	mindMap, err := paperservice.BuildMindMap(c.Request.Context(), ownerID, c.Param("id"))
	if err != nil {
		writePaperErr(c, err, "生成脑图失败")
		return
	}
	response.OK(c, mindMap)
}

func GetMindMap(c *gin.Context) {
	ownerID := tenant.MustStudentID(c.Request.Context())
	mindMap, err := paperservice.GetMindMap(c.Request.Context(), ownerID, c.Param("id"))
	if err != nil {
		writePaperErr(c, err, "查询脑图失败")
		return
	}
	response.OK(c, mindMap)
}

func UpdateMindMap(c *gin.Context) {
	mindMapID, ok := parseMindMapID(c)
	if !ok {
		return
	}
	var req dto.MindMapUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	ownerID := tenant.MustStudentID(c.Request.Context())
	mindMap, err := paperservice.UpdateMindMap(c.Request.Context(), ownerID, mindMapID, req.Graph)
	if err != nil {
		writePaperErr(c, err, "保存脑图失败")
		return
	}
	response.OK(c, mindMap)
}

func SyncMindMap(c *gin.Context) {
	mindMapID, ok := parseMindMapID(c)
	if !ok {
		return
	}
	ownerID := tenant.MustStudentID(c.Request.Context())
	mindMap, err := paperservice.SyncMindMap(c.Request.Context(), ownerID, mindMapID)
	if err != nil {
		writePaperErr(c, err, "同步脑图失败")
		return
	}
	response.OK(c, mindMap)
}

func normalizeRequestPaperIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func parseAnnotationID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("annotation_id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "批注 ID 无效")
		return 0, false
	}
	return id, true
}

func parseMindMapID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("mind_map_id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "脑图 ID 无效")
		return 0, false
	}
	return id, true
}

func parseCompareReportID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("report_id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "对比报告 ID 无效")
		return 0, false
	}
	return id, true
}

func dtoRectsToModel(rects []dto.AnnotationRect) model.AnnotationRects {
	out := make(model.AnnotationRects, 0, len(rects))
	for _, r := range rects {
		out = append(out, dtoRectToModel(r))
	}
	return out
}

func dtoRectToModel(r dto.AnnotationRect) model.AnnotationRect {
	return model.AnnotationRect{
		X1:         r.X1,
		Y1:         r.Y1,
		X2:         r.X2,
		Y2:         r.Y2,
		Width:      r.Width,
		Height:     r.Height,
		PageNumber: r.PageNumber,
	}
}

func writePaperAccessErr(c *gin.Context, err error, fallback string) bool {
	switch {
	case errors.Is(err, errs.ErrPaperNotFound), errors.Is(err, errs.ErrPaperForbidden), errors.Is(err, errs.ErrAnnotationNotFound), errors.Is(err, errs.ErrMindMapNotFound):
		writePaperErr(c, err, fallback)
		return true
	default:
		return false
	}
}

// writePaperErr 把论文错误映射为对应 HTTP 状态。
func writePaperErr(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, errs.ErrPaperNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, errs.ErrAnnotationNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, errs.ErrMindMapNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, errs.ErrPaperFlowNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, errs.ErrMindMapInvalid):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, errs.ErrReportNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, errs.ErrPaperForbidden):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, errs.ErrPaperNotReady):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, errs.ErrPaperBusy):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, errs.ErrPaperFileMissing):
		response.Fail(c, http.StatusNotFound, err.Error())
	default:
		zlog.Error("论文接口错误", "err", err)
		response.Fail(c, http.StatusInternalServerError, fallback)
	}
}
