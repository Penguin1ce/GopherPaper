package graph

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	paperdao "GopherPaper/internal/dao/paper"
	graphstore "GopherPaper/internal/graph"
	"GopherPaper/internal/response"
	"GopherPaper/internal/service/graphsemantic"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/errs"
)

var errPaperMetaNotFound = errors.New("paper meta not found")

// Overview 返回当前用户图谱的规模总览。
// GET /api/v1/graph/overview
//
// @Summary 图谱规模总览
// @Description 返回当前用户知识图谱的论文、作者、关键词、引用数量与年份跨度。
// @Tags graph
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.Response{data=dto.GraphStats}
// @Failure 500 {object} dto.Response
// @Router /graph/overview [get]
func Overview(c *gin.Context) {
	owner := tenant.MustStudentID(c.Request.Context())
	stats, err := graphstore.Overview(c.Request.Context(), owner)
	if err != nil {
		zlog.Error("查询图谱总览失败", "owner", owner, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询图谱总览失败")
		return
	}
	response.OK(c, stats)
}

// Trends 返回研究趋势：每年论文数与 Top 关键词的逐年热度。
// GET /api/v1/graph/trends?keyword_top=10
//
// @Summary 研究趋势
// @Description 返回每年论文数与 Top 关键词逐年热度。
// @Tags graph
// @Produce json
// @Security BearerAuth
// @Param keyword_top query int false "关键词 Top N" default(10)
// @Success 200 {object} dto.Response{data=dto.GraphTrendsResponse}
// @Failure 500 {object} dto.Response
// @Router /graph/trends [get]
func Trends(c *gin.Context) {
	owner := tenant.MustStudentID(c.Request.Context())
	topN := queryInt(c, "keyword_top", 10)
	byYear, err := graphstore.TrendByYear(c.Request.Context(), owner)
	if err != nil {
		zlog.Error("查询年度趋势失败", "owner", owner, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询研究趋势失败")
		return
	}
	keywordTrend, err := graphstore.KeywordTrend(c.Request.Context(), owner, topN)
	if err != nil {
		zlog.Error("查询关键词趋势失败", "owner", owner, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询研究趋势失败")
		return
	}
	response.OK(c, gin.H{"by_year": byYear, "keyword_trend": keywordTrend})
}

// Related 返回与指定论文相关的论文。
// GET /api/v1/graph/papers/:id/related?limit=10
//
// @Summary 查询相关论文
// @Description 返回与指定论文相关的论文，关系来自共享作者、关键词、共被引、直接引用与语义相似边。
// @Tags graph
// @Produce json
// @Security BearerAuth
// @Param id path string true "论文 ID"
// @Param limit query int false "返回数量" default(10)
// @Success 200 {object} dto.Response{data=[]dto.RelatedPaper}
// @Failure 400 {object} dto.Response
// @Failure 500 {object} dto.Response
// @Router /graph/papers/{id}/related [get]
func Related(c *gin.Context) {
	owner := tenant.MustStudentID(c.Request.Context())
	paperID := c.Param("id")
	if paperID == "" {
		response.Fail(c, http.StatusBadRequest, "缺少论文 id")
		return
	}
	limit := queryInt(c, "limit", 10)
	related, err := graphstore.RelatedPapers(c.Request.Context(), owner, paperID, limit)
	if err != nil {
		zlog.Error("查询相关论文失败", "owner", owner, "paper_id", paperID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询相关论文失败")
		return
	}
	response.OK(c, related)
}

// PaperGraph 返回单篇论文中心的元信息知识图谱。查询前会尝试从 MySQL 元信息修复 Neo4j。
// GET /api/v1/graph/papers/:id
func PaperGraph(c *gin.Context) {
	owner := tenant.MustStudentID(c.Request.Context())
	paperID := c.Param("id")
	if paperID == "" {
		response.Fail(c, http.StatusBadRequest, "缺少论文 id")
		return
	}
	if repairErr := repairPaperGraphFromMeta(c.Request.Context(), owner, paperID); repairErr != nil {
		zlog.Error("从 MySQL 元信息同步论文图谱失败", "owner", owner, "paper_id", paperID, "err", repairErr)
	}
	g, err := graphstore.PaperEntityGraph(c.Request.Context(), owner, paperID)
	if err != nil {
		zlog.Error("查询论文知识图谱失败", "owner", owner, "paper_id", paperID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询论文知识图谱失败")
		return
	}
	response.OK(c, g)
}

// Network 返回总览图谱：论文通过共享作者、关键词、机构相连。
// GET /api/v1/graph/network
func Network(c *gin.Context) {
	owner := tenant.MustStudentID(c.Request.Context())
	repairOwnerGraphsFromMeta(c.Request.Context(), owner)
	g, err := graphstore.OverviewEntityGraph(c.Request.Context(), owner)
	if err != nil {
		zlog.Error("查询总览知识图谱失败", "owner", owner, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询总览知识图谱失败")
		return
	}
	response.OK(c, g)
}

// RebuildNetwork 从 MySQL 元信息重建当前用户全部论文图谱，并返回刷新后的总览图谱。
// POST /api/v1/graph/network/rebuild
func RebuildNetwork(c *gin.Context) {
	owner := tenant.MustStudentID(c.Request.Context())
	if err := rebuildOwnerGraphsFromMeta(c.Request.Context(), owner); err != nil {
		zlog.Error("手动重建总览知识图谱失败", "owner", owner, "err", err)
		response.Fail(c, http.StatusInternalServerError, "重建总览知识图谱失败")
		return
	}
	g, err := graphstore.OverviewEntityGraph(c.Request.Context(), owner)
	if err != nil {
		zlog.Error("查询重建后的总览知识图谱失败", "owner", owner, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询总览知识图谱失败")
		return
	}
	response.OK(c, g)
}

// RebuildPaper 从 MySQL 元信息重建单篇论文图谱，并返回刷新后的论文中心图谱。
// POST /api/v1/graph/papers/:id/rebuild
func RebuildPaper(c *gin.Context) {
	owner := tenant.MustStudentID(c.Request.Context())
	paperID := c.Param("id")
	if paperID == "" {
		response.Fail(c, http.StatusBadRequest, "缺少论文 id")
		return
	}
	if err := rebuildPaperGraphFromMeta(c.Request.Context(), owner, paperID); err != nil {
		zlog.Error("手动重建论文知识图谱失败", "owner", owner, "paper_id", paperID, "err", err)
		response.Fail(c, statusForRepairErr(err), messageForRepairErr(err))
		return
	}
	g, err := graphstore.PaperEntityGraph(c.Request.Context(), owner, paperID)
	if err != nil {
		zlog.Error("查询重建后的论文知识图谱失败", "owner", owner, "paper_id", paperID, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询论文知识图谱失败")
		return
	}
	response.OK(c, g)
}

func repairOwnerGraphsFromMeta(ctx context.Context, owner string) {
	papers, err := paperdao.List(ctx, owner)
	if err != nil {
		zlog.Error("同步总览图谱前查询论文列表失败", "owner", owner, "err", err)
		return
	}
	for _, p := range papers {
		if err := repairPaperGraphFromMeta(ctx, owner, p.ID); err != nil {
			zlog.Error("同步单篇论文图谱失败", "owner", owner, "paper_id", p.ID, "err", err)
		}
	}
}

func repairPaperGraphFromMeta(ctx context.Context, owner, paperID string) error {
	pg, err := buildPaperGraphFromMeta(ctx, owner, paperID)
	if err != nil {
		return err
	}
	pg = graphsemantic.PreparePaperGraph(ctx, pg)
	return graphstore.UpsertPaperMetadata(ctx, pg)
}

func rebuildOwnerGraphsFromMeta(ctx context.Context, owner string) error {
	papers, err := paperdao.List(ctx, owner)
	if err != nil {
		return err
	}
	var firstErr error
	for _, p := range papers {
		if err := rebuildPaperGraphFromMeta(ctx, owner, p.ID); err != nil {
			zlog.Error("从 MySQL 元信息重建论文图谱失败", "owner", owner, "paper_id", p.ID, "err", err)
			if errors.Is(err, errs.ErrPaperNotFound) || errors.Is(err, errPaperMetaNotFound) {
				continue
			}
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func rebuildPaperGraphFromMeta(ctx context.Context, owner, paperID string) error {
	pg, err := buildPaperGraphFromMeta(ctx, owner, paperID)
	if err != nil {
		return err
	}
	pg = graphsemantic.PreparePaperGraph(ctx, pg)
	if err := graphstore.UpsertPaperMetadata(ctx, pg); err != nil {
		return err
	}
	return graphsemantic.RefreshPaper(ctx, owner, paperID)
}

func buildPaperGraphFromMeta(ctx context.Context, owner, paperID string) (graphstore.PaperGraph, error) {
	p, err := paperdao.Get(ctx, paperID)
	if err != nil {
		return graphstore.PaperGraph{}, err
	}
	if p.OwnerID != owner {
		return graphstore.PaperGraph{}, errs.ErrPaperForbidden
	}
	meta, err := paperdao.GetMeta(ctx, paperID)
	if err != nil {
		if errors.Is(err, errs.ErrPaperNotFound) {
			return graphstore.PaperGraph{}, errPaperMetaNotFound
		}
		return graphstore.PaperGraph{}, err
	}
	return graphstore.PaperGraph{
		Owner:             owner,
		ID:                paperID,
		Title:             fallbackPaperTitle(p.Title, p.FileName, paperID),
		Year:              meta.PublishYear,
		Venue:             meta.Venue,
		Authors:           []string(meta.Authors),
		Keywords:          []string(meta.Keywords),
		Affiliations:      []string(meta.Affiliations),
		ResearchQuestions: []string(meta.ResearchQuestions),
		Methods:           meta.Methods,
		Experiments:       meta.Experiments,
		Results:           meta.Results,
		Innovations:       []string(meta.Innovations),
		Limitations:       []string(meta.Limitations),
		FutureWork:        []string(meta.FutureWork),
	}, nil
}

func statusForRepairErr(err error) int {
	switch {
	case errors.Is(err, errs.ErrPaperNotFound), errors.Is(err, errPaperMetaNotFound):
		return http.StatusNotFound
	case errors.Is(err, errs.ErrPaperForbidden):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

func messageForRepairErr(err error) string {
	switch {
	case errors.Is(err, errPaperMetaNotFound):
		return "paper_metas 中没有该论文的元信息，请确认解析已完成且 paper_id 与论文 id 一致"
	case errors.Is(err, errs.ErrPaperNotFound):
		return "papers 中没有该论文记录"
	case errors.Is(err, errs.ErrPaperForbidden):
		return "无权访问该论文"
	default:
		return "重建论文知识图谱失败"
	}
}

func fallbackPaperTitle(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return "未命名论文"
}

// Keywords 返回当前用户最热门的关键词。
// GET /api/v1/graph/keywords?top=20
//
// @Summary 热门关键词
// @Description 返回当前用户知识图谱中出现次数最多的关键词。
// @Tags graph
// @Produce json
// @Security BearerAuth
// @Param top query int false "关键词 Top N" default(20)
// @Success 200 {object} dto.Response{data=[]dto.NameCount}
// @Failure 500 {object} dto.Response
// @Router /graph/keywords [get]
func Keywords(c *gin.Context) {
	owner := tenant.MustStudentID(c.Request.Context())
	topN := queryInt(c, "top", 20)
	keywords, err := graphstore.TopKeywords(c.Request.Context(), owner, topN)
	if err != nil {
		zlog.Error("查询热门关键词失败", "owner", owner, "err", err)
		response.Fail(c, http.StatusInternalServerError, "查询热门关键词失败")
		return
	}
	response.OK(c, keywords)
}

func queryInt(c *gin.Context, key string, def int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
