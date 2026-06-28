// toolkit semanticscholar.go 是 Semantic Scholar 学术检索 function tool:相比 arXiv 元数据更丰富,
// 带引用数、年份与开放获取 PDF 链接,适合"按影响力找经典文献""跨预印本/会议录找论文"。
//
// key 可选:配 [tools] semantic_scholar_api_key 走专属额度,留空走公共额度(限流紧、偶发 429)。
// 返回的 pdf_url 优先用 openAccessPdf,缺失时回退由 arXiv 编号拼的 arxiv 直链;两者均落在
// download_paper 的学术站白名单内,模型可直接转交 download_paper 下载并解析入库。
package toolkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// s2BaseURL Semantic Scholar 论文检索端点,测试时可替换。
var s2BaseURL = "https://api.semanticscholar.org/graph/v1/paper/search"

// s2AuthBearer 标记鉴权头形式:false 用官方的 x-api-key 头,true 用 Authorization: Bearer
// (中转代理约定)。由 setS2Endpoints 在配置了代理基址时置真。
var s2AuthBearer bool

// setS2Endpoints 把三个 S2 端点改写到 base 之下并切到 Bearer 鉴权,供配置了中转代理时调用。
// base 形如 https://s2api.ominiai.cn/s2,按官方相同的路径结构拼出 search/paper/recommend 三端点;
// 代理通常能给更宽的额度,故同时把全进程限速放宽(仍留轻量突发护栏)。
func setS2Endpoints(base string) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return
	}
	s2BaseURL = base + "/graph/v1/paper/search"
	s2PaperBaseURL = base + "/graph/v1/paper"
	s2RecommendBaseURL = base + "/recommendations/v1/papers/forpaper"
	s2AuthBearer = true
	s2MinInterval = 300 * time.Millisecond
	// 代理为扛 S2 限流会内部排队/重试,单次响应常 5-7s 还偶发更久,20s 易超时,放宽到 60s。
	s2Client.Timeout = 60 * time.Second
}

// s2Fields 向 S2 请求的字段集,只取展示与下载用得到的;venue/publicationVenue 用来识别顶会。
// paperId 透出供 recommend_similar_papers / get_paper_citations / get_paper_references 顺链定位。
const s2Fields = "paperId,title,abstract,year,authors,citationCount,externalIds,openAccessPdf,venue,publicationVenue"

var s2Client = &http.Client{Timeout: 20 * time.Second}

// s2RetryDelays 是撞 429 后的退避重试节奏:S2 即便带 key 也约 1 req/s,突发必 429,
// 逐步拉长重试间隔多数能在一两拍内放行;留作变量便于测试覆盖缩短。
var s2RetryDelays = []time.Duration{2 * time.Second, 4 * time.Second, 6 * time.Second}

// s2MinInterval 是全进程对 S2 的最小请求间隔:公共额度约 1 req/s,而先锋者一轮 ReAct
// 可能连发 search+references+citations 多次调用,不限速必突发撞 429。配 key 后可适度调小。
// 留作变量便于测试覆盖缩短。
var s2MinInterval = 1100 * time.Millisecond

// s2 限速器状态:s2NextSlot 记录下一个可发请求的时刻,并发调用各自预占一个未来槽位排队。
var (
	s2RateMu   sync.Mutex
	s2NextSlot time.Time
)

// s2Throttle 阻塞到本次请求的限速槽位,保证全进程对 S2 的请求间隔不小于 s2MinInterval。
// 并发调用按到达顺序各占一个递增的未来槽位,互不挤占;ctx 取消则提前返回。
func s2Throttle(ctx context.Context) error {
	s2RateMu.Lock()
	now := time.Now()
	slot := s2NextSlot
	if slot.Before(now) {
		slot = now
	}
	s2NextSlot = slot.Add(s2MinInterval)
	s2RateMu.Unlock()

	wait := time.Until(slot)
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type s2Input struct {
	Query          string `json:"query" jsonschema:"description=检索词,只放 1~3 个核心学术术语,按相关度召回,堆叠越多概念召回越差;用规范英文术语而非口语意译(写 machine-generated text detection 而非 AI generated text);先用最核心词宽搜再据结果加修饰收窄,别一上来就拼 4+ 概念的长句。年份/会议/领域不要塞进这里,用下面的专门参数,required"`
	MaxResults     int    `json:"max_results,omitempty" jsonschema:"description=返回条数,默认 5,上限 10"`
	Year           string `json:"year,omitempty" jsonschema:"description=按年份过滤,支持区间:单年 2025、闭区间 2024-2026、起始 2024-(2024 至今)、截止 -2020;找近期论文先用 current_time 取当前年再据此填"`
	Venue          string `json:"venue,omitempty" jsonschema:"description=按发表会议/期刊过滤,逗号分隔,填常见缩写即可,如 NeurIPS,ICML,CVPR,ICLR,ACL,EMNLP;找顶会论文时填这个,服务端兜底由本工具按别名模糊匹配,别在 query 里凑会议名"`
	FieldsOfStudy  string `json:"fields_of_study,omitempty" jsonschema:"description=按学科领域过滤,逗号分隔,取值如 Computer Science、Mathematics、Biology、Medicine、Physics;锁定领域用这个"`
	OpenAccessOnly bool   `json:"open_access_only,omitempty" jsonschema:"description=只返回有可下载 pdf_url(含 arXiv 镜像兜底)的论文;仅当用户明确只要能下载/导入的论文时才设 true。注意顶会论文很多在 S2 无开放全文但有 arXiv 版,默认不要设 true 以免漏掉它们"`
}

type s2Paper struct {
	PaperID       string   `json:"paper_id,omitempty" jsonschema:"description=Semantic Scholar 论文 ID,可直接传给 recommend_similar_papers/get_paper_citations/get_paper_references 顺链找相似论文、后续引用与参考文献"`
	Title         string   `json:"title" jsonschema:"description=论文标题"`
	Authors       []string `json:"authors" jsonschema:"description=作者列表"`
	Year          int      `json:"year,omitempty" jsonschema:"description=发表年份"`
	Venue         string   `json:"venue,omitempty" jsonschema:"description=发表的会议或期刊,如 NeurIPS、CVPR、ACL;据此判断是否顶会,为空说明 S2 无收录场所信息(可能仅预印本)"`
	CitationCount int      `json:"citation_count" jsonschema:"description=被引次数,越高影响力越大,可据此判断经典程度"`
	Abstract      string   `json:"abstract,omitempty" jsonschema:"description=摘要,部分论文 S2 无摘要时为空"`
	ArxivID       string   `json:"arxiv_id,omitempty" jsonschema:"description=arXiv 编号,有则说明该文也在 arXiv"`
	DOI           string   `json:"doi,omitempty" jsonschema:"description=DOI"`
	PDFURL        string   `json:"pdf_url,omitempty" jsonschema:"description=开放获取 PDF 直链,非空时可直接传给 download_paper;为空说明无开放全文,无法自动下载"`
	BibTeX        string   `json:"bibtex" jsonschema:"description=BibTeX 引用条目,用户要引用该文时可直接复制进 .bib 文件;有 venue 时按会议/期刊归类"`
}

type s2Output struct {
	Papers []s2Paper `json:"papers" jsonschema:"description=检索到的论文列表,按相关度排序;pdf_url 为空的条目无开放全文不能下载"`
}

// s2Resp 是 S2 search 的响应,只取我们用到的字段。
type s2Resp struct {
	Data  []s2PaperRec `json:"data"`
	Error string       `json:"error"`
}

// s2PaperRec 是 S2 各端点返回的论文记录公共子集,search/citations/references/recommendations 共用。
type s2PaperRec struct {
	PaperID          string `json:"paperId"`
	Title            string `json:"title"`
	Abstract         string `json:"abstract"`
	Year             int    `json:"year"`
	CitationCount    int    `json:"citationCount"`
	Venue            string `json:"venue"`
	PublicationVenue struct {
		Name string `json:"name"`
	} `json:"publicationVenue"`
	Authors []struct {
		Name string `json:"name"`
	} `json:"authors"`
	ExternalIDs struct {
		ArXiv string `json:"ArXiv"`
		DOI   string `json:"DOI"`
	} `json:"externalIds"`
	OpenAccessPdf struct {
		URL string `json:"url"`
	} `json:"openAccessPdf"`
}

// toPaper 把 S2 原始记录转成对外 s2Paper:venue 取规范名优先、pdf_url 缺失时回退 arXiv 直链、拼 BibTeX。
func (d s2PaperRec) toPaper() s2Paper {
	authors := make([]string, 0, len(d.Authors))
	for _, a := range d.Authors {
		if a.Name != "" {
			authors = append(authors, a.Name)
		}
	}
	// pdf_url 优先用开放获取直链,缺失时回退 arXiv 编号拼的 arxiv 直链,都落在 download_paper 白名单内。
	pdfURL := strings.TrimSpace(d.OpenAccessPdf.URL)
	if pdfURL == "" && d.ExternalIDs.ArXiv != "" {
		pdfURL = "https://arxiv.org/pdf/" + d.ExternalIDs.ArXiv
	}
	// venue 优先用规范化的 publicationVenue.name,缺失时回退自由文本 venue 字段。
	venue := strings.TrimSpace(d.PublicationVenue.Name)
	if venue == "" {
		venue = strings.TrimSpace(d.Venue)
	}
	return s2Paper{
		PaperID:       d.PaperID,
		Title:         d.Title,
		Authors:       authors,
		Year:          d.Year,
		Venue:         venue,
		CitationCount: d.CitationCount,
		Abstract:      d.Abstract,
		ArxivID:       d.ExternalIDs.ArXiv,
		DOI:           d.ExternalIDs.DOI,
		PDFURL:        pdfURL,
		BibTeX: generateBibTeX(bibEntry{
			Title: d.Title, Authors: authors, Year: d.Year,
			Venue: venue, ArxivID: d.ExternalIDs.ArXiv, DOI: d.ExternalIDs.DOI,
		}),
	}
}

// errS2NotFound 标识 S2 返回 404(论文标识无收录),供调用方给出可读提示而非裸 404。
var errS2NotFound = errors.New("论文不存在或 Semantic Scholar 无收录该标识")

// s2Get 对 Semantic Scholar 任意端点发 GET 并把 JSON 解进 out,收口统一的 UA、key 注入与
// 429/500 退避重试(节奏同 s2RetryDelays);404 返回 errS2NotFound。search 与顺链工具共用。
func s2Get(ctx context.Context, apiKey, reqURL string, out any) error {
	for attempt := 0; ; attempt++ {
		// 发请求前先过全进程限速,避免突发把公共额度打到 429。
		if err := s2Throttle(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return fmt.Errorf("构建请求失败: %w", err)
		}
		setAcademicHeaders(req)
		if apiKey != "" {
			// 中转代理用 Authorization: Bearer,官方用 x-api-key。
			if s2AuthBearer {
				req.Header.Set("Authorization", "Bearer "+apiKey)
			} else {
				req.Header.Set("x-api-key", apiKey)
			}
		}
		resp, err := s2Client.Do(req)
		if err != nil {
			return fmt.Errorf("请求 Semantic Scholar 失败: %w", err)
		}
		status := resp.StatusCode
		// 429 限流与 500 临时服务端错误都可退避重试:逐步拉长间隔多数能在一两拍内放行。
		if (status == http.StatusTooManyRequests || status == http.StatusInternalServerError) && attempt < len(s2RetryDelays) {
			delay := s2Jitter(retryAfterDelay(resp, s2RetryDelays[attempt]))
			resp.Body.Close()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		switch {
		case status == http.StatusTooManyRequests:
			return fmt.Errorf("Semantic Scholar 持续限流(429),已重试 %d 次仍失败,稍后再试或改用 search_arxiv", len(s2RetryDelays))
		case status == http.StatusInternalServerError:
			return fmt.Errorf("Semantic Scholar 服务端错误(500),已重试 %d 次仍失败,稍后再试", len(s2RetryDelays))
		case status == http.StatusNotFound:
			return errS2NotFound
		case status != http.StatusOK:
			return fmt.Errorf("Semantic Scholar 返回 %d: %s", status, strings.TrimSpace(string(body)))
		case readErr != nil:
			return fmt.Errorf("读取响应失败: %w", readErr)
		}
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("解析响应失败: %w", err)
		}
		return nil
	}
}

// s2Jitter 给退避时长加 0~25% 的随机抖动,避免多个并发调用退避后同拍重试再次撞 429。
func s2Jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	return d + time.Duration(rand.Int63n(int64(d)/4+1))
}

// clampResults 把请求条数夹到 [1, max],非正数回退 def。
func clampResults(n, def, max int) int {
	if n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

// newSemanticScholarTool 构建 Semantic Scholar 检索工具,apiKey 可空(走公共额度)。
func newSemanticScholarTool(apiKey string) tool.Tool {
	fn := func(ctx context.Context, in s2Input) (s2Output, error) {
		return s2Search(ctx, apiKey, in)
	}
	return function.NewFunctionTool(fn,
		function.WithName("search_semantic_scholar"),
		function.WithDescription("在 Semantic Scholar 上检索论文,带发表会议/期刊(venue)、被引次数、年份与开放获取 pdf_url。支持 year(年份区间)、venue(会议列表)、fields_of_study(学科)、open_access_only 等结构化过滤参数,在服务端精确筛——补引用数/相似论文用本工具,别把会议名/年份塞进 query。某会议某年份的录用身份必须先由 search_conference_proceedings / search_openreview_papers 等官方源确认;本工具只作补充,不能单独证明会议录用。pdf_url 非空的可直接传给 download_paper 下载并解析入库。结果里的 paper_id 可继续传给 recommend_similar_papers(找相似论文)、get_paper_citations(找后续引用)、get_paper_references(找参考文献)顺藤摸瓜。"),
	)
}

// s2VenueAliases 把常见顶会缩写映射到其会在 S2 venue/publicationVenue.name 里出现的匹配片段
// (含缩写本身与全称关键片段),用于客户端模糊匹配。表外的 venue 退化为按原词小写匹配。
var s2VenueAliases = map[string][]string{
	"neurips":         {"neurips", "nips", "neural information processing systems"},
	"icml":            {"icml", "international conference on machine learning"},
	"iclr":            {"iclr", "international conference on learning representations"},
	"cvpr":            {"cvpr", "computer vision and pattern recognition"},
	"iccv":            {"iccv", "international conference on computer vision"},
	"eccv":            {"eccv", "european conference on computer vision"},
	"acl":             {"acl", "association for computational linguistics"},
	"emnlp":           {"emnlp", "empirical methods in natural language processing"},
	"naacl":           {"naacl", "north american chapter of the association for computational linguistics"},
	"aaai":            {"aaai", "association for the advancement of artificial intelligence"},
	"ijcai":           {"ijcai", "international joint conference on artificial intelligence"},
	"kdd":             {"kdd", "knowledge discovery and data mining"},
	"sigir":           {"sigir"},
	"www":             {"www", "world wide web", "the web conference"},
	"interspeech":     {"interspeech"},
	"icassp":          {"icassp", "acoustics, speech and signal processing"},
	"usenix security": {"usenix security"},
	"usenix":          {"usenix"},
	"ccs":             {"ccs", "computer and communications security"},
	"s&p":             {"s&p", "ieee symposium on security and privacy", "security and privacy"},
	"sp":              {"s&p", "ieee symposium on security and privacy", "security and privacy"},
	"oakland":         {"security and privacy", "oakland"},
	"ndss":            {"ndss", "network and distributed system security"},
	"raid":            {"raid", "research in attacks, intrusions"},
	"acsac":           {"acsac", "annual computer security applications"},
}

// splitVenues 把逗号分隔的 venue 入参切成小写去空的 token 列表。
func splitVenues(s string) []string {
	out := []string{}
	for _, v := range strings.Split(s, ",") {
		if v = strings.ToLower(strings.TrimSpace(v)); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// matchesVenue 判断论文场所串是否命中任一目标 venue:目标经别名展开后做小写子串匹配。
func matchesVenue(paperVenue string, wanted []string) bool {
	pv := strings.ToLower(paperVenue)
	if strings.TrimSpace(pv) == "" {
		return false
	}
	for _, tok := range wanted {
		forms := s2VenueAliases[tok]
		if forms == nil {
			forms = []string{tok}
		}
		for _, f := range forms {
			if strings.Contains(pv, f) {
				return true
			}
		}
	}
	return false
}

func s2Search(ctx context.Context, apiKey string, in s2Input) (s2Output, error) {
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return s2Output{}, fmt.Errorf("search_semantic_scholar: query 不能为空")
	}
	n := clampResults(in.MaxResults, 5, 10)
	// 要"可下载"或按 venue 过滤时都走客户端筛(原因见下),故多取候选(过滤会砍掉一部分)再截断到 n。
	wantVenues := splitVenues(in.Venue)
	limit := n
	if in.OpenAccessOnly || len(wantVenues) > 0 {
		limit = min(n*4, 100) // S2 单页上限 100
	}
	params := url.Values{}
	params.Set("query", q)
	params.Set("limit", fmt.Sprint(limit))
	params.Set("fields", s2Fields)
	// year/fields_of_study 直接透传给 S2 服务端精确筛,省得模型在 query 里凑关键词。
	// venue 不透传:S2 的 venue 过滤是对库内 venue 字符串精确匹配,而模型填的是缩写
	// (NeurIPS/ICML/...),库里却常存全称(Neural Information Processing Systems 等),
	// 精确匹配几乎必空。改为多取候选后在客户端按缩写别名模糊匹配 venue + publicationVenue.name。
	if y := strings.TrimSpace(in.Year); y != "" {
		params.Set("year", y)
	}
	if f := strings.TrimSpace(in.FieldsOfStudy); f != "" {
		params.Set("fieldsOfStudy", f)
	}

	var r s2Resp
	if err := s2Get(ctx, apiKey, s2BaseURL+"?"+params.Encode(), &r); err != nil {
		return s2Output{}, fmt.Errorf("search_semantic_scholar: %w", err)
	}

	papers := make([]s2Paper, 0, len(r.Data))
	for _, d := range r.Data {
		p := d.toPaper()
		// open_access_only:按兜底后的最终 pdf_url 是否可下载来筛,而非 S2 的 openAccessPdf 字段。
		if in.OpenAccessOnly && p.PDFURL == "" {
			continue
		}
		// venue 客户端过滤:匹配规范名与原始 venue 任一,按缩写别名模糊命中即保留。
		if len(wantVenues) > 0 && !matchesVenue(d.PublicationVenue.Name+" "+d.Venue, wantVenues) {
			continue
		}
		papers = append(papers, p)
	}
	// 多取的候选过滤后截断回 n。
	if len(papers) > n {
		papers = papers[:n]
	}
	return s2Output{Papers: papers}, nil
}
