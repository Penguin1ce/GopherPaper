// toolkit openalex.go 是 OpenAlex 学术检索 function tool:覆盖 2.5 亿+ 文献的开放学术图谱,
// 带引用数、年份、venue、DOI 与开放获取 PDF 链接,作 Semantic Scholar 的冗余/平替——
// OpenAlex 限流远松(免费 key 每天 $1 额度),先锋者一轮 ReAct 多次检索不易撞 S2 的 429。
//
// key 必填:OpenAlex 现按调用计费,key 经 api_key 查询参数透传(留空则 toolkit 不登记本工具)。
// 返回的 pdf_url 取 best_oa_location / open_access 的开放获取直链,命中 download_paper 白名单时
// (如 arxiv、ncbi、aclanthology)可直接转交 download_paper 下载并解析入库;非白名单站则只供用户点开。
package toolkit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// openAlexBaseURL OpenAlex works 检索端点,测试时可替换。
var openAlexBaseURL = "https://api.openalex.org/works"

var openAlexClient = &http.Client{Timeout: 20 * time.Second}

// openAlexRetryDelays 是撞 429 后的退避重试节奏。OpenAlex 限流松,极少触发,留作兜底。
var openAlexRetryDelays = []time.Duration{time.Second, 2 * time.Second}

// openAlexFields 向 OpenAlex 请求的字段集,只取展示与下载用得到的,缩小响应体。
const openAlexFields = "id,doi,title,publication_year,publication_date,authorships,cited_by_count,primary_location,best_oa_location,open_access,abstract_inverted_index"

type openAlexInput struct {
	Query      string `json:"query" jsonschema:"description=检索词,只放主题关键词;年份/排序用下面的专门参数,不要塞进这里,required"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"description=返回条数,默认 5,上限 10"`
	FromYear   int    `json:"from_year,omitempty" jsonschema:"description=起始年份(含),按发表时间过滤;找近期论文先用 current_time 取当前年再据此填"`
	ToYear     int    `json:"to_year,omitempty" jsonschema:"description=截止年份(含),按发表时间过滤;留空且填了 from_year 则不设上界"`
	Sort       string `json:"sort,omitempty" jsonschema:"description=排序方式:relevance 按相关度(默认),citations 按被引数从高到低(找经典/高影响力),recency 按发表时间从新到旧(找最新)"`
}

type openAlexPaper struct {
	OpenAlexID   string   `json:"openalex_id" jsonschema:"description=OpenAlex 作品 ID,如 W2741809807"`
	DOI          string   `json:"doi,omitempty" jsonschema:"description=DOI,如 10.1145/3292500.3330701,可喂引用工具或 Unpaywall 取 OA"`
	Title        string   `json:"title" jsonschema:"description=论文标题"`
	Authors      []string `json:"authors" jsonschema:"description=作者列表"`
	Abstract     string   `json:"abstract,omitempty" jsonschema:"description=摘要,由 OpenAlex 倒排索引还原"`
	Year         int      `json:"year,omitempty" jsonschema:"description=发表年份"`
	Venue        string   `json:"venue,omitempty" jsonschema:"description=发表来源(期刊/会议名)"`
	CitedByCount int      `json:"cited_by_count" jsonschema:"description=被引次数,衡量影响力"`
	PDFURL       string   `json:"pdf_url,omitempty" jsonschema:"description=开放获取 PDF 直链,命中 download_paper 白名单时可直接下载入库"`
	LandingURL   string   `json:"landing_url,omitempty" jsonschema:"description=论文落地页链接,供用户点开查看"`
	BibTeX       string   `json:"bibtex" jsonschema:"description=BibTeX 引用条目,用户要引用该文时可直接复制进 .bib 文件"`
}

type openAlexOutput struct {
	Papers []openAlexPaper `json:"papers" jsonschema:"description=检索到的论文列表"`
}

// openAlexResponse 是 OpenAlex works 响应,只取我们用到的字段。
type openAlexResponse struct {
	Results []openAlexWork `json:"results"`
}

type openAlexWork struct {
	ID              string `json:"id"`  // https://openalex.org/W2741809807
	DOI             string `json:"doi"` // https://doi.org/10.1145/...
	Title           string `json:"title"`
	PublicationYear int    `json:"publication_year"`
	CitedByCount    int    `json:"cited_by_count"`
	Authorships     []struct {
		Author struct {
			DisplayName string `json:"display_name"`
		} `json:"author"`
	} `json:"authorships"`
	PrimaryLocation openAlexLocation `json:"primary_location"`
	BestOALocation  openAlexLocation `json:"best_oa_location"`
	OpenAccess      struct {
		OAURL string `json:"oa_url"`
	} `json:"open_access"`
	// AbstractInverted 是词到出现位置列表的倒排索引,OpenAlex 不直接给纯文本摘要。
	AbstractInverted map[string][]int `json:"abstract_inverted_index"`
}

type openAlexLocation struct {
	PDFURL         string `json:"pdf_url"`
	LandingPageURL string `json:"landing_page_url"`
	Source         struct {
		DisplayName string `json:"display_name"`
	} `json:"source"`
}

// newOpenAlexTool 构建 OpenAlex 检索工具,key 经 api_key 查询参数透传。
func newOpenAlexTool(apiKey string) tool.Tool {
	fn := func(ctx context.Context, in openAlexInput) (openAlexOutput, error) {
		return openAlexSearch(ctx, apiKey, in)
	}
	return function.NewFunctionTool(fn,
		function.WithName("search_openalex"),
		function.WithDescription("在 OpenAlex 开放学术图谱上检索论文,覆盖跨学科 2.5 亿+ 文献,返回标题、作者、摘要、被引数、venue、DOI 与开放获取 pdf_url。支持 from_year/to_year(发表年份区间)、sort(citations 按被引、recency 按时间)等过滤,别把年份/排序塞进 query。与 search_semantic_scholar 定位相近但限流更松,可作其平替或冗余:S2 撞 429 或想找高被引经典时优先用本工具,sort=citations 找经典、sort=recency 找最新。拿到的 pdf_url 若是 arXiv/PMC/ACL 等白名单学术站直链,可直接传给 download_paper 下载并解析入库。"),
	)
}

func openAlexSearch(ctx context.Context, apiKey string, in openAlexInput) (openAlexOutput, error) {
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return openAlexOutput{}, fmt.Errorf("search_openalex: query 不能为空")
	}
	n := in.MaxResults
	if n <= 0 {
		n = 5
	}
	if n > 10 {
		n = 10
	}

	params := url.Values{}
	params.Set("search", q)
	params.Set("per-page", fmt.Sprint(n))
	params.Set("select", openAlexFields)
	if apiKey != "" {
		params.Set("api_key", apiKey)
	}
	// 年份区间走 filter,与 search 叠加。
	if f := openAlexYearFilter(in.FromYear, in.ToYear); f != "" {
		params.Set("filter", f)
	}
	// citations 按被引降序,recency 按发表时间降序,否则交服务端按相关度。
	switch strings.ToLower(strings.TrimSpace(in.Sort)) {
	case "citations":
		params.Set("sort", "cited_by_count:desc")
	case "recency":
		params.Set("sort", "publication_date:desc")
	}

	reqURL := openAlexBaseURL + "?" + params.Encode()
	var data openAlexResponse
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return openAlexOutput{}, fmt.Errorf("search_openalex: 构建请求失败: %w", err)
		}
		setAcademicHeaders(req)
		resp, err := openAlexClient.Do(req)
		if err != nil {
			return openAlexOutput{}, fmt.Errorf("search_openalex: 请求 OpenAlex 失败: %w", err)
		}
		status := resp.StatusCode
		if status == http.StatusTooManyRequests && attempt < len(openAlexRetryDelays) {
			delay := retryAfterDelay(resp, openAlexRetryDelays[attempt])
			resp.Body.Close()
			select {
			case <-ctx.Done():
				return openAlexOutput{}, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		if status != http.StatusOK {
			resp.Body.Close()
			return openAlexOutput{}, fmt.Errorf("search_openalex: OpenAlex 返回 %d", status)
		}
		data = openAlexResponse{}
		err = json.NewDecoder(resp.Body).Decode(&data)
		resp.Body.Close()
		if err != nil {
			return openAlexOutput{}, fmt.Errorf("search_openalex: 解析响应失败: %w", err)
		}
		break
	}

	papers := make([]openAlexPaper, 0, len(data.Results))
	for _, w := range data.Results {
		authors := make([]string, 0, len(w.Authorships))
		for _, a := range w.Authorships {
			if name := strings.TrimSpace(a.Author.DisplayName); name != "" {
				authors = append(authors, name)
			}
		}
		doi := openAlexDOI(w.DOI)
		title := strings.TrimSpace(w.Title)
		// PDF 直链优先 best_oa / primary 的 pdf_url,缺则回退 open_access.oa_url(可能是落地页)。
		pdfURL := firstNonEmpty(w.BestOALocation.PDFURL, w.PrimaryLocation.PDFURL, w.OpenAccess.OAURL)
		landing := firstNonEmpty(w.PrimaryLocation.LandingPageURL, w.BestOALocation.LandingPageURL)
		venue := firstNonEmpty(w.PrimaryLocation.Source.DisplayName, w.BestOALocation.Source.DisplayName)
		papers = append(papers, openAlexPaper{
			OpenAlexID:   openAlexShortID(w.ID),
			DOI:          doi,
			Title:        title,
			Authors:      authors,
			Abstract:     openAlexAbstract(w.AbstractInverted),
			Year:         w.PublicationYear,
			Venue:        venue,
			CitedByCount: w.CitedByCount,
			PDFURL:       pdfURL,
			LandingURL:   landing,
			BibTeX: generateBibTeX(bibEntry{
				Title: title, Authors: authors, Year: w.PublicationYear, DOI: doi,
			}),
		})
	}
	return openAlexOutput{Papers: papers}, nil
}

// openAlexYearFilter 把起止年份拼成 OpenAlex filter 子句(逗号即 AND),两端都为 0 返回空串。
func openAlexYearFilter(fromYear, toYear int) string {
	parts := make([]string, 0, 2)
	if fromYear > 0 {
		parts = append(parts, fmt.Sprintf("from_publication_date:%04d-01-01", fromYear))
	}
	if toYear > 0 {
		parts = append(parts, fmt.Sprintf("to_publication_date:%04d-12-31", toYear))
	}
	return strings.Join(parts, ",")
}

// openAlexShortID 从 id URL(https://openalex.org/W2741809807)抽出短 ID,失败原样返回。
func openAlexShortID(raw string) string {
	raw = strings.TrimSpace(raw)
	if i := strings.LastIndex(raw, "/"); i >= 0 {
		return raw[i+1:]
	}
	return raw
}

// openAlexDOI 把 https://doi.org/<doi> 形式归一为裸 DOI,无则返回空。
func openAlexDOI(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if _, after, found := strings.Cut(raw, "doi.org/"); found {
		return after
	}
	return raw
}

// openAlexAbstract 把 OpenAlex 的倒排索引(词→位置列表)还原成纯文本摘要。
func openAlexAbstract(inverted map[string][]int) string {
	if len(inverted) == 0 {
		return ""
	}
	type wp struct {
		word string
		pos  int
	}
	words := make([]wp, 0)
	for w, positions := range inverted {
		for _, p := range positions {
			words = append(words, wp{word: w, pos: p})
		}
	}
	sort.Slice(words, func(i, j int) bool { return words[i].pos < words[j].pos })
	parts := make([]string, len(words))
	for i, w := range words {
		parts[i] = w.word
	}
	return strings.Join(parts, " ")
}

// firstNonEmpty 返回第一个非空白字符串,全空返回空串。
func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if t := strings.TrimSpace(s); t != "" {
			return t
		}
	}
	return ""
}
