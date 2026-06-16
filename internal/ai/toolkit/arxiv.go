// toolkit arxiv.go 是 arXiv 学术检索 function tool:按关键词/标题/作者查 arXiv 论文,
// 返回标题、作者、摘要、发表时间与可直接下载的 pdf_url。免费、无需 key,走 arXiv 官方 Atom API。
//
// 与 download_paper 闭环:返回的 pdf_url 形如 https://arxiv.org/pdf/<id>,命中 download_paper
// 的学术站白名单,模型拿到后可直接传给 download_paper 下载并解析入库。
package toolkit

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// arxivBaseURL arXiv 官方查询端点,测试时可替换。
var arxivBaseURL = "https://export.arxiv.org/api/query"

var arxivClient = &http.Client{Timeout: 20 * time.Second}

type arxivInput struct {
	Query      string `json:"query" jsonschema:"description=检索词,只放主题关键词或字段语法(如 ti:transformer au:vaswani);年份/分类不要塞进这里,用下面的专门参数,required"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"description=返回条数,默认 5,上限 10"`
	FromYear   int    `json:"from_year,omitempty" jsonschema:"description=起始年份(含),按论文提交时间过滤;找近期论文先用 current_time 取当前年再据此填,如近两年填去年"`
	ToYear     int    `json:"to_year,omitempty" jsonschema:"description=截止年份(含),按论文提交时间过滤;留空且填了 from_year 则不设上界"`
	Categories string `json:"categories,omitempty" jsonschema:"description=按 arXiv 学科分类过滤,逗号分隔,如 cs.AI,cs.CV,cs.CL,cs.LG;比把领域塞进关键词更准"`
	Sort       string `json:"sort,omitempty" jsonschema:"description=排序方式:relevance 按相关度(默认),recency 按提交时间从新到旧;用户要最新/近期论文时用 recency"`
}

type arxivPaper struct {
	ArxivID   string   `json:"arxiv_id" jsonschema:"description=arXiv 编号(含版本号),如 2503.03480v1"`
	Title     string   `json:"title" jsonschema:"description=论文标题"`
	Authors   []string `json:"authors" jsonschema:"description=作者列表"`
	Abstract  string   `json:"abstract" jsonschema:"description=摘要"`
	Published string   `json:"published" jsonschema:"description=首次发表时间,如 2025-03-05"`
	PDFURL    string   `json:"pdf_url" jsonschema:"description=PDF 直链,可直接传给 download_paper 下载并解析入库"`
	AbsURL    string   `json:"abs_url" jsonschema:"description=arXiv 摘要页链接,供用户点开查看"`
	BibTeX    string   `json:"bibtex" jsonschema:"description=BibTeX 引用条目,用户要引用该文时可直接复制进 .bib 文件"`
}

type arxivOutput struct {
	Papers []arxivPaper `json:"papers" jsonschema:"description=检索到的论文列表,按相关度排序"`
}

// arxivFeed 是 arXiv Atom 响应,只取我们用到的字段。
type arxivFeed struct {
	XMLName xml.Name     `xml:"feed"`
	Entries []arxivEntry `xml:"entry"`
}

type arxivEntry struct {
	ID        string `xml:"id"` // http://arxiv.org/abs/2503.03480v1
	Title     string `xml:"title"`
	Summary   string `xml:"summary"`
	Published string `xml:"published"` // 2025-03-05T18:59:59Z
	Authors   []struct {
		Name string `xml:"name"`
	} `xml:"author"`
}

// newArxivTool 构建 arXiv 检索工具,无外部 key。
func newArxivTool() tool.Tool {
	fn := func(ctx context.Context, in arxivInput) (arxivOutput, error) {
		return arxivSearch(ctx, in)
	}
	return function.NewFunctionTool(fn,
		function.WithName("search_arxiv"),
		function.WithDescription("在 arXiv 上检索论文,返回标题、作者、摘要、发表时间与可下载的 pdf_url。支持 from_year/to_year(提交时间区间)、categories(学科分类)、sort(recency 按时间从新到旧)等过滤参数,别把年份/分类塞进 query。用户想找某方向/某作者的论文、要最新预印本时调用,找最新用 sort=recency;arXiv 是预印本库,venue 信号弱,要顶会论文优先用 search_semantic_scholar。拿到的 pdf_url 可直接传给 download_paper 下载并解析入库。"),
	)
}

func arxivSearch(ctx context.Context, in arxivInput) (arxivOutput, error) {
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return arxivOutput{}, fmt.Errorf("search_arxiv: query 不能为空")
	}
	n := in.MaxResults
	if n <= 0 {
		n = 5
	}
	if n > 10 {
		n = 10
	}
	// 未带字段前缀时用 all: 全字段检索;带前缀(ti:/au: 等)则原样透传。
	base := q
	if !strings.Contains(q, ":") {
		base = "all:" + q
	}
	// 主题 + 分类 + 提交时间区间用 AND 串成 arXiv 检索式,分类过滤交服务端。
	clauses := []string{base}
	if cat := arxivCategoryClause(in.Categories); cat != "" {
		clauses = append(clauses, cat)
	}
	if date := arxivDateClause(in.FromYear, in.ToYear); date != "" {
		clauses = append(clauses, date)
	}
	params := url.Values{}
	params.Set("search_query", strings.Join(clauses, " AND "))
	params.Set("start", "0")
	params.Set("max_results", fmt.Sprint(n))
	// recency 按提交时间从新到旧,适合「找最新」;否则按相关度。
	if strings.EqualFold(strings.TrimSpace(in.Sort), "recency") {
		params.Set("sortBy", "submittedDate")
		params.Set("sortOrder", "descending")
	} else {
		params.Set("sortBy", "relevance")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, arxivBaseURL+"?"+params.Encode(), nil)
	if err != nil {
		return arxivOutput{}, fmt.Errorf("search_arxiv: 构建请求失败: %w", err)
	}
	resp, err := arxivClient.Do(req)
	if err != nil {
		return arxivOutput{}, fmt.Errorf("search_arxiv: 请求 arXiv 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return arxivOutput{}, fmt.Errorf("search_arxiv: arXiv 返回 %d", resp.StatusCode)
	}

	var feed arxivFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return arxivOutput{}, fmt.Errorf("search_arxiv: 解析响应失败: %w", err)
	}

	papers := make([]arxivPaper, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		id := arxivIDFromURL(e.ID)
		if id == "" {
			continue
		}
		authors := make([]string, 0, len(e.Authors))
		for _, a := range e.Authors {
			if name := collapseWS(a.Name); name != "" {
				authors = append(authors, name)
			}
		}
		title := collapseWS(e.Title)
		published := arxivDate(e.Published)
		papers = append(papers, arxivPaper{
			ArxivID:   id,
			Title:     title,
			Authors:   authors,
			Abstract:  collapseWS(e.Summary),
			Published: published,
			PDFURL:    "https://arxiv.org/pdf/" + id,
			AbsURL:    "https://arxiv.org/abs/" + id,
			BibTeX: generateBibTeX(bibEntry{
				Title: title, Authors: authors, Year: yearFromDate(published), ArxivID: id,
			}),
		})
	}
	return arxivOutput{Papers: papers}, nil
}

// arxivCategoryClause 把逗号分隔的分类拼成 (cat:cs.AI OR cat:cs.CV) 子句,空则返回空串。
func arxivCategoryClause(raw string) string {
	cats := make([]string, 0)
	for c := range strings.SplitSeq(raw, ",") {
		if c = strings.TrimSpace(c); c != "" {
			cats = append(cats, "cat:"+c)
		}
	}
	if len(cats) == 0 {
		return ""
	}
	if len(cats) == 1 {
		return cats[0]
	}
	return "(" + strings.Join(cats, " OR ") + ")"
}

// arxivDateClause 把起止年份拼成 submittedDate:[YYYYMMDDhhmm TO YYYYMMDDhhmm] 子句。
// 只给一端时另一端取宽边界(1900 / 2100);两端都为 0 返回空串。
func arxivDateClause(fromYear, toYear int) string {
	if fromYear <= 0 && toYear <= 0 {
		return ""
	}
	from, to := fromYear, toYear
	if from <= 0 {
		from = 1900
	}
	if to <= 0 {
		to = 2100
	}
	return fmt.Sprintf("submittedDate:[%04d01010000 TO %04d12312359]", from, to)
}

// arxivIDFromURL 从 entry.id(http://arxiv.org/abs/2503.03480v1)抽出编号,失败返回空。
func arxivIDFromURL(raw string) string {
	const marker = "/abs/"
	if i := strings.LastIndex(raw, marker); i >= 0 {
		return strings.TrimSpace(raw[i+len(marker):])
	}
	return ""
}

// arxivDate 取 RFC3339 时间的日期段,解析失败原样返回。
func arxivDate(s string) string {
	if t, err := time.Parse(time.RFC3339, strings.TrimSpace(s)); err == nil {
		return t.Format("2006-01-02")
	}
	return strings.TrimSpace(s)
}

// yearFromDate 从 2006-01-02 形式的日期取年份,失败返回 0。
func yearFromDate(s string) int {
	if len(s) >= 4 {
		if y, err := strconv.Atoi(s[:4]); err == nil {
			return y
		}
	}
	return 0
}

// collapseWS 把 arXiv 标题/摘要里的换行与多余空白折叠成单空格。
func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
