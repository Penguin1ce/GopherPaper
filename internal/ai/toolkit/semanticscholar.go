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
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// s2BaseURL Semantic Scholar 论文检索端点,测试时可替换。
var s2BaseURL = "https://api.semanticscholar.org/graph/v1/paper/search"

// s2Fields 向 S2 请求的字段集,只取展示与下载用得到的;venue/publicationVenue 用来识别顶会。
const s2Fields = "title,abstract,year,authors,citationCount,externalIds,openAccessPdf,venue,publicationVenue"

var s2Client = &http.Client{Timeout: 20 * time.Second}

// s2RetryDelays 是撞 429 后的退避重试节奏:S2 即便带 key 也约 1 req/s,突发必 429,
// 逐步拉长重试间隔多数能在一两拍内放行;留作变量便于测试覆盖缩短。
var s2RetryDelays = []time.Duration{time.Second, 2 * time.Second, 3 * time.Second}

type s2Input struct {
	Query          string `json:"query" jsonschema:"description=检索词,只放主题关键词,如 contrastive learning for sentence embeddings;年份/会议/领域不要塞进这里,用下面的专门参数,required"`
	MaxResults     int    `json:"max_results,omitempty" jsonschema:"description=返回条数,默认 5,上限 10"`
	Year           string `json:"year,omitempty" jsonschema:"description=按年份过滤,支持区间:单年 2025、闭区间 2024-2026、起始 2024-(2024 至今)、截止 -2020;找近期论文先用 current_time 取当前年再据此填"`
	Venue          string `json:"venue,omitempty" jsonschema:"description=按发表会议/期刊过滤,逗号分隔,填常见缩写即可,如 NeurIPS,ICML,CVPR,ICLR,ACL,EMNLP;找顶会论文时填这个,服务端兜底由本工具按别名模糊匹配,别在 query 里凑会议名"`
	FieldsOfStudy  string `json:"fields_of_study,omitempty" jsonschema:"description=按学科领域过滤,逗号分隔,取值如 Computer Science、Mathematics、Biology、Medicine、Physics;锁定领域用这个"`
	OpenAccessOnly bool   `json:"open_access_only,omitempty" jsonschema:"description=只返回有可下载 pdf_url(含 arXiv 镜像兜底)的论文;仅当用户明确只要能下载/导入的论文时才设 true。注意顶会论文很多在 S2 无开放全文但有 arXiv 版,默认不要设 true 以免漏掉它们"`
}

type s2Paper struct {
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
	Data []struct {
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
	} `json:"data"`
	Error string `json:"error"`
}

// newSemanticScholarTool 构建 Semantic Scholar 检索工具,apiKey 可空(走公共额度)。
func newSemanticScholarTool(apiKey string) tool.Tool {
	fn := func(ctx context.Context, in s2Input) (s2Output, error) {
		return s2Search(ctx, apiKey, in)
	}
	return function.NewFunctionTool(fn,
		function.WithName("search_semantic_scholar"),
		function.WithDescription("在 Semantic Scholar 上检索论文,带发表会议/期刊(venue)、被引次数、年份与开放获取 pdf_url。支持 year(年份区间)、venue(会议列表)、fields_of_study(学科)、open_access_only 等结构化过滤参数,在服务端精确筛——找顶会论文用 venue、找近期论文用 year,别把会议名/年份塞进 query。要找顶会论文、按影响力找经典文献、跨 arXiv/会议录/期刊找论文时优先用本工具;pdf_url 非空的可直接传给 download_paper 下载并解析入库。"),
	)
}

// s2VenueAliases 把常见顶会缩写映射到其会在 S2 venue/publicationVenue.name 里出现的匹配片段
// (含缩写本身与全称关键片段),用于客户端模糊匹配。表外的 venue 退化为按原词小写匹配。
var s2VenueAliases = map[string][]string{
	"neurips":          {"neurips", "nips", "neural information processing systems"},
	"icml":             {"icml", "international conference on machine learning"},
	"iclr":             {"iclr", "international conference on learning representations"},
	"cvpr":             {"cvpr", "computer vision and pattern recognition"},
	"iccv":             {"iccv", "international conference on computer vision"},
	"eccv":             {"eccv", "european conference on computer vision"},
	"acl":              {"acl", "association for computational linguistics"},
	"emnlp":            {"emnlp", "empirical methods in natural language processing"},
	"naacl":            {"naacl", "north american chapter of the association for computational linguistics"},
	"aaai":             {"aaai", "association for the advancement of artificial intelligence"},
	"ijcai":            {"ijcai", "international joint conference on artificial intelligence"},
	"kdd":              {"kdd", "knowledge discovery and data mining"},
	"sigir":            {"sigir"},
	"www":              {"www", "world wide web", "the web conference"},
	"interspeech":      {"interspeech"},
	"icassp":           {"icassp", "acoustics, speech and signal processing"},
	"usenix security":  {"usenix security"},
	"usenix":           {"usenix"},
	"ccs":              {"ccs", "computer and communications security"},
	"s&p":              {"s&p", "ieee symposium on security and privacy", "security and privacy"},
	"sp":               {"s&p", "ieee symposium on security and privacy", "security and privacy"},
	"oakland":          {"security and privacy", "oakland"},
	"ndss":             {"ndss", "network and distributed system security"},
	"raid":             {"raid", "research in attacks, intrusions"},
	"acsac":            {"acsac", "annual computer security applications"},
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
	n := in.MaxResults
	if n <= 0 {
		n = 5
	}
	if n > 10 {
		n = 10
	}
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

	reqURL := s2BaseURL + "?" + params.Encode()
	// 撞 429 就按 s2RetryDelays 退避重试,首请求 + len 次重试;非 429 立即返回。
	var r s2Resp
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return s2Output{}, fmt.Errorf("search_semantic_scholar: 构建请求失败: %w", err)
		}
		setAcademicHeaders(req)
		if apiKey != "" {
			req.Header.Set("x-api-key", apiKey)
		}
		resp, err := s2Client.Do(req)
		if err != nil {
			return s2Output{}, fmt.Errorf("search_semantic_scholar: 请求 Semantic Scholar 失败: %w", err)
		}
		status := resp.StatusCode
		// 429 限流与 500 临时服务端错误都可退避重试:500 通常是 S2 瞬时抖动,重试大多能放行。
		if (status == http.StatusTooManyRequests || status == http.StatusInternalServerError) && attempt < len(s2RetryDelays) {
			delay := retryAfterDelay(resp, s2RetryDelays[attempt])
			resp.Body.Close()
			select {
			case <-ctx.Done():
				return s2Output{}, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		r = s2Resp{}
		err = json.NewDecoder(resp.Body).Decode(&r)
		resp.Body.Close()
		if status == http.StatusTooManyRequests {
			return s2Output{}, fmt.Errorf("search_semantic_scholar: Semantic Scholar 持续限流(429),已重试 %d 次仍失败,稍后再试或改用 search_arxiv", len(s2RetryDelays))
		}
		if status == http.StatusInternalServerError {
			return s2Output{}, fmt.Errorf("search_semantic_scholar: Semantic Scholar 服务端错误(500),已重试 %d 次仍失败,稍后再试或改用 search_arxiv", len(s2RetryDelays))
		}
		if err != nil {
			return s2Output{}, fmt.Errorf("search_semantic_scholar: 解析响应失败: %w", err)
		}
		if status != http.StatusOK {
			return s2Output{}, fmt.Errorf("search_semantic_scholar: Semantic Scholar 返回 %d: %s", status, r.Error)
		}
		break
	}

	papers := make([]s2Paper, 0, len(r.Data))
	for _, d := range r.Data {
		authors := make([]string, 0, len(d.Authors))
		for _, a := range d.Authors {
			if a.Name != "" {
				authors = append(authors, a.Name)
			}
		}
		// pdf_url 优先用开放获取直链,缺失时回退 arXiv 编号拼的 arxiv 直链,都落在白名单内。
		pdfURL := strings.TrimSpace(d.OpenAccessPdf.URL)
		if pdfURL == "" && d.ExternalIDs.ArXiv != "" {
			pdfURL = "https://arxiv.org/pdf/" + d.ExternalIDs.ArXiv
		}
		// open_access_only:按兜底后的最终 pdf_url 是否可下载来筛,而非 S2 的 openAccessPdf 字段。
		if in.OpenAccessOnly && pdfURL == "" {
			continue
		}
		// venue 优先用规范化的 publicationVenue.name,缺失时回退自由文本 venue 字段。
		venue := strings.TrimSpace(d.PublicationVenue.Name)
		if venue == "" {
			venue = strings.TrimSpace(d.Venue)
		}
		// venue 客户端过滤:匹配规范名与原始 venue 任一,按缩写别名模糊命中即保留。
		if len(wantVenues) > 0 && !matchesVenue(d.PublicationVenue.Name+" "+d.Venue, wantVenues) {
			continue
		}
		papers = append(papers, s2Paper{
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
		})
	}
	// 多取的候选过滤后截断回 n。
	if len(papers) > n {
		papers = papers[:n]
	}
	return s2Output{Papers: papers}, nil
}
