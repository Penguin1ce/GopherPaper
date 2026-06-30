// toolkit sciverse.go 是 SciVerse(OpenDataLab)语义检索 function tool:走它的 /agentic-search
// 端点,提交自然语言问题后返回相关文献的正文片段(chunk),每条带页码与文献定位字段。
// 与 search_openalex / search_semantic_scholar 定位互补——后两者按元数据找"有哪些论文",
// 本工具直接召回"论文里说了什么"的证据段落,适合要给出处段落支撑结论的场景。
//
// key 必填:SciVerse 按调用计费、且 initialize 即需鉴权,经 Authorization: Bearer 头透传
// (留空则 toolkit 不登记本工具,不裸调)。文献由 MinerU 解析,与本站解析同源。
package toolkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"

	"GopherPaper/internal/zlog"
)

// sciVerseBaseURL SciVerse 语义检索端点,测试时可替换。
var sciVerseBaseURL = "https://api.sciverse.space/agentic-search"

// sciVerseContentURL SciVerse 原文续读端点,测试时可替换。
var sciVerseContentURL = "https://api.sciverse.space/content"

// sciVerseContentDefaultLimit 续读默认窗口(Unicode 码点)。比官方默认 700 稍大,
// 一次给足够上下文又不至于把整段全文灌进模型;上限收敛见 readSciVerseContent。
const sciVerseContentDefaultLimit = 1500

// sciVerseContentMaxLimit 单次续读窗口上限,防止把超长全文一次拉回灌爆上下文。
const sciVerseContentMaxLimit = 6000

var sciVerseClient = &http.Client{Timeout: 30 * time.Second}

// sciVerseRetryDelays 撞 429 后的退避重试节奏。
var sciVerseRetryDelays = []time.Duration{time.Second, 2 * time.Second}

// sciVerseImageRe 匹配 chunk 里 MinerU 留下的图片占位 markdown,召回证据段落时剔除以减噪。
var sciVerseImageRe = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)

// sciVerseMinScore 相关度下限:低于此分的命中视为噪声丢弃。SciVerse 总按 top_k 凑满返回,
// 无相关结果时也会塞回最不相关的几条(实测中英混杂或不在库的论文,噪声分集中在 0.5~0.68,
// 真相关结果普遍 0.89+),不设下限会让模型把噪声当证据。设 0.7 把二者切开,宁可返回空也不返噪。
var sciVerseMinScore = 0.7

// sciVerseCJKRe 检测 query 里的中日韩字符。SciVerse 语料以英文为主,中文 query 召回崩坏
// (实测返回完全离题结果),命中即告警,便于回看模型是否违反"先译英文再检索"的纪律。
var sciVerseCJKRe = regexp.MustCompile(`[\p{Han}\p{Hiragana}\p{Katakana}\p{Hangul}]`)

type sciVerseInput struct {
	Query      string `json:"query" jsonschema:"description=检索问题,用自然语言描述要找的内容;只放主题与问题本身,不要塞年份/排序等过滤词,required"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"description=返回片段条数,默认 5,上限 10"`
}

type sciVerseHit struct {
	Title        string  `json:"title" jsonschema:"description=论文标题"`
	Snippet      string  `json:"snippet" jsonschema:"description=命中的正文片段,即可直接引用的证据段落"`
	Abstract     string  `json:"abstract,omitempty" jsonschema:"description=论文摘要"`
	Year         int     `json:"year,omitempty" jsonschema:"description=发表年份"`
	Venue        string  `json:"venue,omitempty" jsonschema:"description=发表来源(期刊/会议名)"`
	CitedByCount int     `json:"cited_by_count" jsonschema:"description=被引次数,衡量影响力"`
	Topic        string  `json:"topic,omitempty" jsonschema:"description=主题领域"`
	PageNo       int     `json:"page_no,omitempty" jsonschema:"description=片段所在原文页码,可作出处"`
	DocID        string  `json:"doc_id,omitempty" jsonschema:"description=文献在 SciVerse 内的稳定标识,连同 offset 传给 read_sciverse_content 可续读该处前后更多原文"`
	Offset       int     `json:"offset" jsonschema:"description=该片段在全文中的起始偏移(Unicode 码点),连同 doc_id 传给 read_sciverse_content 续读上下文"`
	Score        float64 `json:"score" jsonschema:"description=相关度得分,越高越相关"`
}

type sciVerseOutput struct {
	Hits []sciVerseHit `json:"hits" jsonschema:"description=检索到的文献片段列表,按相关度排序"`
}

// sciVerseRequest 是 /agentic-search 请求体,只填我们用到的字段。
type sciVerseRequest struct {
	Query     string `json:"query"`
	TopK      int    `json:"top_k"`
	Retrieval string `json:"retrieval"`
}

// sciVerseResponse 是 /agentic-search 响应,只取我们用到的字段。
type sciVerseResponse struct {
	BizCode int              `json:"biz_code"`
	Code    string           `json:"code"`
	Message string           `json:"message"`
	Hits    []sciVerseRawHit `json:"hits"`
}

type sciVerseRawHit struct {
	Title string `json:"title"`
	// Chunk 命中的正文片段,含 MinerU 图片占位,展示前剔除。
	Chunk        string  `json:"chunk"`
	Abstract     string  `json:"abstract"`
	Year         int     `json:"publication_published_year"`
	Venue        string  `json:"publication_venue_name_unified"`
	CitedByCount int     `json:"citation_count"`
	Topic        string  `json:"primary_topic"`
	PageNo       int     `json:"page_no"`
	DocID        string  `json:"doc_id"`
	Offset       int     `json:"offset"`
	Score        float64 `json:"score"`
	// author 字段服务端返回字节数组编码的乱码,故不取。
}

// newSciVerseTool 构建 SciVerse 语义检索工具,key 经 Authorization: Bearer 头透传。
func newSciVerseTool(apiKey string) tool.Tool {
	fn := func(ctx context.Context, in sciVerseInput) (sciVerseOutput, error) {
		return sciVerseSearch(ctx, apiKey, in)
	}
	return function.NewFunctionTool(fn,
		function.WithName("search_sciverse"),
		function.WithDescription("在 SciVerse 学术语料上做语义检索,提交自然语言问题后直接召回相关论文的正文片段(snippet),每条带标题、页码、被引数、venue 与相关度。与 search_openalex / search_semantic_scholar 互补:那两个按元数据找'有哪些论文',本工具直接给出'论文里怎么说'的证据段落,适合需要带出处段落支撑结论、或想读到具体论述而非只看标题摘要时优先用。query 用自然语言描述问题即可,不要塞年份/排序等过滤词。"),
	)
}

func sciVerseSearch(ctx context.Context, apiKey string, in sciVerseInput) (sciVerseOutput, error) {
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return sciVerseOutput{}, fmt.Errorf("search_sciverse: query 不能为空")
	}
	n := in.MaxResults
	if n <= 0 {
		n = 5
	}
	if n > 10 {
		n = 10
	}

	// hybrid 为向量+关键词融合召回,综合效果最好,固定走 hybrid 不暴露给模型选。
	payload, err := json.Marshal(sciVerseRequest{Query: q, TopK: n, Retrieval: "hybrid"})
	if err != nil {
		return sciVerseOutput{}, fmt.Errorf("search_sciverse: 构建请求体失败: %w", err)
	}
	// 记录实际下发的 query 与参数:模型常把检索不到归因于工具,但多数是 query 太杂/带中文/
	// 拿语义检索当指定论文查找用导致的,先把真实 query 打出来才能判因。
	if sciVerseCJKRe.MatchString(q) {
		zlog.Warn("search_sciverse query 含中日韩字符,召回会显著变差", "query", q)
	}
	zlog.Info("search_sciverse 请求", "query", q, "top_k", n)
	start := time.Now()

	var data sciVerseResponse
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, sciVerseBaseURL, bytes.NewReader(payload))
		if err != nil {
			return sciVerseOutput{}, fmt.Errorf("search_sciverse: 构建请求失败: %w", err)
		}
		setAcademicHeaders(req)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := sciVerseClient.Do(req)
		if err != nil {
			return sciVerseOutput{}, fmt.Errorf("search_sciverse: 请求 SciVerse 失败: %w", err)
		}
		status := resp.StatusCode
		if status == http.StatusTooManyRequests && attempt < len(sciVerseRetryDelays) {
			delay := retryAfterDelay(resp, sciVerseRetryDelays[attempt])
			resp.Body.Close()
			select {
			case <-ctx.Done():
				return sciVerseOutput{}, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		if status != http.StatusOK {
			resp.Body.Close()
			return sciVerseOutput{}, fmt.Errorf("search_sciverse: SciVerse 返回 %d", status)
		}
		data = sciVerseResponse{}
		err = json.NewDecoder(resp.Body).Decode(&data)
		resp.Body.Close()
		if err != nil {
			return sciVerseOutput{}, fmt.Errorf("search_sciverse: 解析响应失败: %w", err)
		}
		break
	}
	if data.BizCode != 0 {
		return sciVerseOutput{}, fmt.Errorf("search_sciverse: SciVerse 业务错误 %d %s: %s", data.BizCode, data.Code, data.Message)
	}

	hits := make([]sciVerseHit, 0, len(data.Hits))
	dropped := 0
	for _, h := range data.Hits {
		// 低于相关度下限的命中是 top_k 凑数的噪声,丢弃,避免模型把离题片段当证据。
		if h.Score < sciVerseMinScore {
			dropped++
			continue
		}
		hits = append(hits, sciVerseHit{
			Title:        strings.TrimSpace(h.Title),
			Snippet:      sciVerseCleanChunk(h.Chunk),
			Abstract:     strings.TrimSpace(h.Abstract),
			Year:         h.Year,
			Venue:        strings.TrimSpace(h.Venue),
			CitedByCount: h.CitedByCount,
			Topic:        strings.TrimSpace(h.Topic),
			PageNo:       h.PageNo,
			DocID:        h.DocID,
			Offset:       h.Offset,
			Score:        h.Score,
		})
	}
	// 命中数为 0 时单独 Warn:含中文/带具体论文名当语义查找/论文不在库,都会落到这里;
	// raw 是过滤前的原始条数,raw>0 而 hits=0 即"返回了但全是低分噪声",便于事后回看判因。
	if len(hits) == 0 {
		zlog.Warn("search_sciverse 零命中", "query", q, "top_k", n,
			"raw", len(data.Hits), "dropped_lowscore", dropped, "cost_ms", time.Since(start).Milliseconds())
		return sciVerseOutput{Hits: hits}, nil
	}
	// 打出每条命中的标题与 score,直接看目标论文是否在召回里、相关度高低。
	titles := make([]string, 0, len(hits))
	for _, h := range hits {
		titles = append(titles, fmt.Sprintf("%.3f|%s", h.Score, sciVerseTruncate(h.Title, 60)))
	}
	zlog.Info("search_sciverse 返回", "query", q, "hits", len(hits), "dropped_lowscore", dropped,
		"cost_ms", time.Since(start).Milliseconds(), "titles", titles)
	return sciVerseOutput{Hits: hits}, nil
}

// sciVerseTruncate 截断字符串到 n 个 rune,超出加省略号,只用于日志。
func sciVerseTruncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// --- read_sciverse_content:按 doc_id + offset 续读原文,展开 search_sciverse 片段的上下文 ---

type sciVerseContentInput struct {
	DocID  string `json:"doc_id" jsonschema:"description=文献标识,取自 search_sciverse 返回的 doc_id,required"`
	Offset int    `json:"offset,omitempty" jsonschema:"description=起始偏移(Unicode 码点),通常取 search_sciverse 命中片段的 offset;想读片段之前的内容就减去一段。省略则从全文开头读"`
	Limit  int    `json:"limit,omitempty" jsonschema:"description=本次最多读取的字符数(Unicode 码点),默认 1500,上限 6000;不够再以返回的 next_offset 继续读"`
}

type sciVerseContentOutput struct {
	Text       string `json:"text" jsonschema:"description=读到的原文片段"`
	NextOffset int    `json:"next_offset" jsonschema:"description=下次续读应传入的 offset"`
	More       bool   `json:"more" jsonschema:"description=是否仍有后续内容可读;true 时可用 next_offset 继续调用"`
	TextLength int    `json:"text_length" jsonschema:"description=该文献全文总字符数(Unicode 码点)"`
}

// sciVerseContentResponse 是 /content 响应。
type sciVerseContentResponse struct {
	Text       string `json:"text"`
	NextOffset int    `json:"next_offset"`
	More       bool   `json:"more"`
	TextLength int    `json:"text_length"`
}

// newReadSciVerseContentTool 构建续读工具,key 经 Authorization: Bearer 头透传。
func newReadSciVerseContentTool(apiKey string) tool.Tool {
	fn := func(ctx context.Context, in sciVerseContentInput) (sciVerseContentOutput, error) {
		return readSciVerseContent(ctx, apiKey, in)
	}
	return function.NewFunctionTool(fn,
		function.WithName("read_sciverse_content"),
		function.WithDescription("按 doc_id 续读 SciVerse 文献原文,展开 search_sciverse 召回片段周围的上下文。典型链路:先 search_sciverse 命中相关片段,若 snippet 被截断或想看片段前后更完整的论述,就把该命中的 doc_id 与 offset 传进来读取该处更多原文;返回的 next_offset/more 可用于继续往后读。只适用于 search_sciverse 检索到的文献(它返回 doc_id),不能凭空对任意论文用;要读本站已导入论文用 search_my_papers。"),
	)
}

func readSciVerseContent(ctx context.Context, apiKey string, in sciVerseContentInput) (sciVerseContentOutput, error) {
	docID := strings.TrimSpace(in.DocID)
	if docID == "" {
		return sciVerseContentOutput{}, fmt.Errorf("read_sciverse_content: doc_id 不能为空")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = sciVerseContentDefaultLimit
	}
	if limit > sciVerseContentMaxLimit {
		limit = sciVerseContentMaxLimit
	}
	offset := max(in.Offset, 0)

	params := url.Values{}
	params.Set("doc_id", docID)
	// 始终走分页模式(传 offset)并带 limit,避免省略 offset 时拉回整段全文灌爆上下文。
	params.Set("offset", fmt.Sprint(offset))
	params.Set("limit", fmt.Sprint(limit))
	reqURL := sciVerseContentURL + "?" + params.Encode()

	zlog.Info("read_sciverse_content 请求", "doc_id", docID, "offset", offset, "limit", limit)
	var data sciVerseContentResponse
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return sciVerseContentOutput{}, fmt.Errorf("read_sciverse_content: 构建请求失败: %w", err)
		}
		setAcademicHeaders(req)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := sciVerseClient.Do(req)
		if err != nil {
			return sciVerseContentOutput{}, fmt.Errorf("read_sciverse_content: 请求 SciVerse 失败: %w", err)
		}
		status := resp.StatusCode
		if status == http.StatusTooManyRequests && attempt < len(sciVerseRetryDelays) {
			delay := retryAfterDelay(resp, sciVerseRetryDelays[attempt])
			resp.Body.Close()
			select {
			case <-ctx.Done():
				return sciVerseContentOutput{}, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		if status != http.StatusOK {
			resp.Body.Close()
			return sciVerseContentOutput{}, fmt.Errorf("read_sciverse_content: SciVerse 返回 %d", status)
		}
		data = sciVerseContentResponse{}
		err = json.NewDecoder(resp.Body).Decode(&data)
		resp.Body.Close()
		if err != nil {
			return sciVerseContentOutput{}, fmt.Errorf("read_sciverse_content: 解析响应失败: %w", err)
		}
		break
	}

	return sciVerseContentOutput{
		Text:       sciVerseCleanChunk(data.Text),
		NextOffset: data.NextOffset,
		More:       data.More,
		TextLength: data.TextLength,
	}, nil
}

// sciVerseCleanChunk 剔除 chunk 里的图片占位 markdown 并归一空白,留下纯证据文本。
func sciVerseCleanChunk(s string) string {
	s = sciVerseImageRe.ReplaceAllString(s, "")
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}
