// Package graph 处理知识图谱查询的 HTTP 接口:总览、研究趋势、相关论文、热门关键词。
// 处理函数为裸包级 func,owner 取自鉴权注入的 tenant,直接委托 internal/graph 查询,
// 数据按用户隔离。
package graph

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	graphstore "GopherPaper/internal/graph"
	"GopherPaper/internal/response"
	"GopherPaper/internal/tenant"
	"GopherPaper/internal/zlog"
)

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

// Trends 返回研究趋势:每年论文数与 Top 关键词的逐年热度。
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

// Related 返回与指定论文相关的论文,关系来自共享作者/关键词/共被引/直接引用。
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

// queryInt 取整型查询参数,缺失或非法返回默认值。
func queryInt(c *gin.Context, key string, def int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
