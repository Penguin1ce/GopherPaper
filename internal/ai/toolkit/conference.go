// toolkit conference.go 提供官方会议论文检索工具。它补上 Semantic Scholar 的短板:
// 当用户问"某会议某年份有哪些论文"时,先查官方 proceedings / OpenReview,再用 S2/arXiv 补引用数与预印本。
package toolkit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

var (
	conferenceClient          = &http.Client{Timeout: 30 * time.Second}
	neuripsProceedingsBaseURL = "https://proceedings.neurips.cc"
	openReviewAPIBaseURL      = "https://api2.openreview.net"
	openReviewWebBaseURL      = "https://openreview.net"
)

type conferenceProceedingsInput struct {
	Venue      string `json:"venue" jsonschema:"description=会议简称或名称,如 NeurIPS。当前官方 proceedings 支持 NeurIPS;其他会议优先改用 search_openreview_papers 或 search_semantic_scholar,required"`
	Year       int    `json:"year" jsonschema:"description=会议年份,如 2025。用户说 25 年时先用 current_time 判断具体世纪后填 2025,required"`
	Query      string `json:"query,omitempty" jsonschema:"description=主题关键词,如 agent、LLM agent、web agent;不要放年份/会议名"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"description=返回条数,默认 10,上限 30"`
}

type openReviewInput struct {
	Venue      string `json:"venue,omitempty" jsonschema:"description=会议简称或名称,如 NeurIPS、ICLR、ICML。若无法识别,改填 venue_id"`
	Year       int    `json:"year,omitempty" jsonschema:"description=会议年份,如 2025。venue_id 为空时必填"`
	VenueID    string `json:"venue_id,omitempty" jsonschema:"description=OpenReview venueid,如 NeurIPS.cc/2025/Conference、ICLR.cc/2025/Conference;当 venue/year 无法映射时直接填这个"`
	Query      string `json:"query,omitempty" jsonschema:"description=主题关键词,如 agent、LLM agent、web agent;不要放年份/会议名"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"description=返回条数,默认 10,上限 30"`
}

type conferencePaper struct {
	Title     string   `json:"title" jsonschema:"description=论文标题"`
	Authors   []string `json:"authors,omitempty" jsonschema:"description=作者列表"`
	Year      int      `json:"year" jsonschema:"description=会议年份"`
	Venue     string   `json:"venue" jsonschema:"description=会议名称或简称"`
	Track     string   `json:"track,omitempty" jsonschema:"description=官方 track 或接收形态,如 Main Conference Track、Datasets and Benchmarks Track、Poster"`
	Abstract  string   `json:"abstract,omitempty" jsonschema:"description=摘要;官方来源未返回时为空"`
	Keywords  []string `json:"keywords,omitempty" jsonschema:"description=关键词;官方来源未返回时为空"`
	URL       string   `json:"url" jsonschema:"description=官方论文页面链接"`
	PDFURL    string   `json:"pdf_url,omitempty" jsonschema:"description=官方或 OpenReview PDF 直链,非空时可在用户确认后传给 download_paper"`
	Source    string   `json:"source" jsonschema:"description=官方来源名称,如 NeurIPS Proceedings、OpenReview"`
	SourceURL string   `json:"source_url" jsonschema:"description=本次检索的官方列表或 venue 页面"`
	BibTeX    string   `json:"bibtex,omitempty" jsonschema:"description=BibTeX;官方来源返回时提供"`
}

type conferenceSearchOutput struct {
	Papers    []conferencePaper `json:"papers" jsonschema:"description=官方来源检索到的论文列表"`
	Source    string            `json:"source" jsonschema:"description=检索的官方来源名称"`
	SourceURL string            `json:"source_url" jsonschema:"description=检索的官方列表或 venue 页面"`
	Note      string            `json:"note,omitempty" jsonschema:"description=补充说明,如当前 proceedings 支持范围或扫描上限"`
}

// newConferenceProceedingsTool 构建官方 proceedings 检索工具。目前先支持 NeurIPS 官方论文集。
func newConferenceProceedingsTool() tool.Tool {
	fn := func(ctx context.Context, in conferenceProceedingsInput) (conferenceSearchOutput, error) {
		return searchConferenceProceedings(ctx, in)
	}
	return function.NewFunctionTool(fn,
		function.WithName("search_conference_proceedings"),
		function.WithDescription("检索会议官方 proceedings。用户询问“某会议某年份论文/推荐/有哪些”时必须优先调用本工具或 search_openreview_papers,不要只靠 Semantic Scholar。当前支持 NeurIPS 官方 proceedings,返回官方页面、track、作者与可直接导入的 PDF 直链;query 只放主题词或论文标题,年份和会议填专门参数。用户要导入 NeurIPS 论文但手头没有 pdf_url 时,先用本工具按标题补官方 PDF。"),
	)
}

// newOpenReviewTool 构建 OpenReview accepted/public notes 检索工具。
func newOpenReviewTool() tool.Tool {
	fn := func(ctx context.Context, in openReviewInput) (conferenceSearchOutput, error) {
		return searchOpenReviewPapers(ctx, in)
	}
	return function.NewFunctionTool(fn,
		function.WithName("search_openreview_papers"),
		function.WithDescription("检索 OpenReview 上某会议某年份的 accepted/public papers。用户询问“某会议某年份论文/推荐/有哪些”时必须优先调用本工具或 search_conference_proceedings,不要只靠 Semantic Scholar。支持 venue/year 自动映射 NeurIPS、ICLR、ICML,也可直接传 venue_id;query 只放主题词或论文标题,返回官方 OpenReview 链接、可直接导入的 PDF、摘要、关键词与 BibTeX。用户要导入 OpenReview 会议论文但手头没有 pdf_url 时,先用本工具按标题补官方 PDF。"),
	)
}

func searchConferenceProceedings(ctx context.Context, in conferenceProceedingsInput) (conferenceSearchOutput, error) {
	venue := strings.TrimSpace(in.Venue)
	if venue == "" {
		return conferenceSearchOutput{}, fmt.Errorf("search_conference_proceedings: venue 不能为空")
	}
	if in.Year <= 0 {
		return conferenceSearchOutput{}, fmt.Errorf("search_conference_proceedings: year 不能为空")
	}
	n := clampResults(in.MaxResults, 10, 30)
	if !isNeuripsVenue(venue) {
		return conferenceSearchOutput{}, fmt.Errorf("search_conference_proceedings: 当前官方 proceedings 仅支持 NeurIPS; %s 可先试 search_openreview_papers,或用 search_semantic_scholar 补充", venue)
	}
	sourceURL := strings.TrimRight(neuripsProceedingsBaseURL, "/") + "/paper_files/paper/" + strconv.Itoa(in.Year)
	body, err := academicGetText(ctx, sourceURL)
	if err != nil {
		return conferenceSearchOutput{}, fmt.Errorf("search_conference_proceedings: 请求 NeurIPS proceedings 失败: %w", err)
	}
	papers, err := parseNeuripsProceedings(body, in.Year, sourceURL)
	if err != nil {
		return conferenceSearchOutput{}, fmt.Errorf("search_conference_proceedings: 解析 NeurIPS proceedings 失败: %w", err)
	}
	papers = filterAndRankConferencePapers(papers, in.Query)
	if len(papers) > n {
		papers = papers[:n]
	}
	return conferenceSearchOutput{
		Papers:    papers,
		Source:    "NeurIPS Proceedings",
		SourceURL: sourceURL,
		Note:      "结果来自 NeurIPS 官方 proceedings; 引用数、相似论文可再用 search_semantic_scholar 补充。",
	}, nil
}

func searchOpenReviewPapers(ctx context.Context, in openReviewInput) (conferenceSearchOutput, error) {
	venueID, err := resolveOpenReviewVenueID(in)
	if err != nil {
		return conferenceSearchOutput{}, err
	}
	n := clampResults(in.MaxResults, 10, 30)
	pageSize := 1000
	if strings.TrimSpace(in.Query) == "" {
		pageSize = n
	}

	all := make([]conferencePaper, 0, n)
	scanned := 0
	maxPages := 8
	for page := 0; page < maxPages; page++ {
		offset := page * pageSize
		reqURL := openReviewAPIBaseURL + "/notes?" + url.Values{
			"content.venueid": {venueID},
			"limit":           {strconv.Itoa(pageSize)},
			"offset":          {strconv.Itoa(offset)},
		}.Encode()
		var resp openReviewNotesResp
		if err := academicGetJSON(ctx, reqURL, &resp); err != nil {
			return conferenceSearchOutput{}, fmt.Errorf("search_openreview_papers: 请求 OpenReview 失败: %w", err)
		}
		scanned += len(resp.Notes)
		for _, note := range resp.Notes {
			p := note.toConferencePaper(venueID, in.Year)
			if p.Title == "" {
				continue
			}
			if strings.TrimSpace(in.Query) == "" || conferencePaperScore(p, in.Query) > 0 {
				all = append(all, p)
			}
		}
		if len(resp.Notes) < pageSize || strings.TrimSpace(in.Query) == "" {
			break
		}
	}
	all = filterAndRankConferencePapers(all, in.Query)
	if len(all) > n {
		all = all[:n]
	}
	sourceURL := openReviewWebBaseURL + "/group?id=" + url.QueryEscape(venueID)
	note := fmt.Sprintf("结果来自 OpenReview venueid=%s, 本次扫描 %d 条记录。", venueID, scanned)
	if scanned >= maxPages*pageSize {
		note += " 如结果仍偏少,可换更宽关键词重试。"
	}
	return conferenceSearchOutput{
		Papers:    all,
		Source:    "OpenReview",
		SourceURL: sourceURL,
		Note:      note,
	}, nil
}

func academicGetText(ctx context.Context, reqURL string) (string, error) {
	body, err := academicGetBytes(ctx, reqURL)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func academicGetJSON(ctx context.Context, reqURL string, out any) error {
	body, err := academicGetBytes(ctx, reqURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	return nil
}

func academicGetBytes(ctx context.Context, reqURL string) ([]byte, error) {
	delays := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("构建请求失败: %w", err)
		}
		setAcademicHeaders(req)
		resp, err := conferenceClient.Do(req)
		if err != nil {
			return nil, err
		}
		status := resp.StatusCode
		if (status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable) && attempt < len(delays) {
			delay := retryAfterDelay(resp, delays[attempt])
			resp.Body.Close()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("读取响应失败: %w", readErr)
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("返回 %d: %s", status, strings.TrimSpace(string(body)))
		}
		return body, nil
	}
}

func parseNeuripsProceedings(body string, year int, sourceURL string) ([]conferencePaper, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(neuripsProceedingsBaseURL, "/")
	var papers []conferencePaper
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" && attr(n, "data-track") != "" {
			if p, ok := parseNeuripsItem(n, year, base, sourceURL); ok {
				papers = append(papers, p)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return papers, nil
}

func parseNeuripsItem(n *html.Node, year int, baseURL, sourceURL string) (conferencePaper, bool) {
	link := findDescendant(n, func(x *html.Node) bool {
		return x.Type == html.ElementNode && x.Data == "a" && attr(x, "title") == "paper title"
	})
	if link == nil {
		return conferencePaper{}, false
	}
	href := attr(link, "href")
	title := collapseWS(nodeText(link))
	if title == "" || href == "" {
		return conferencePaper{}, false
	}
	authorsNode := findDescendant(n, func(x *html.Node) bool {
		return x.Type == html.ElementNode && x.Data == "span" && classHas(x, "paper-authors")
	})
	trackNode := findDescendant(n, func(x *html.Node) bool {
		return x.Type == html.ElementNode && x.Data == "span" && classHas(x, "paper-track-badge")
	})
	return conferencePaper{
		Title:     title,
		Authors:   splitAuthors(nodeText(authorsNode)),
		Year:      year,
		Venue:     "NeurIPS",
		Track:     neuripsTrack(href, nodeText(trackNode), attr(n, "data-track")),
		URL:       absolutize(baseURL, href),
		PDFURL:    absolutize(baseURL, neuripsPDFPath(href)),
		Source:    "NeurIPS Proceedings",
		SourceURL: sourceURL,
	}, true
}

func neuripsPDFPath(href string) string {
	p := strings.TrimSpace(href)
	if p == "" {
		return ""
	}
	p = strings.Replace(p, "/hash/", "/file/", 1)
	p = strings.Replace(p, "-Abstract-", "-Paper-", 1)
	p = strings.TrimSuffix(p, ".html") + ".pdf"
	return p
}

func neuripsTrack(href, badge, dataTrack string) string {
	if s := collapseWS(badge); s != "" {
		return s
	}
	if i := strings.Index(href, "-Abstract-"); i >= 0 {
		s := strings.TrimSuffix(href[i+len("-Abstract-"):], ".html")
		return strings.ReplaceAll(s, "_", " ")
	}
	return strings.ReplaceAll(collapseWS(dataTrack), "_", " ")
}

func isNeuripsVenue(venue string) bool {
	v := strings.ToLower(strings.TrimSpace(venue))
	return v == "neurips" || v == "nips" || strings.Contains(v, "neural information processing systems")
}

func resolveOpenReviewVenueID(in openReviewInput) (string, error) {
	if id := strings.TrimSpace(in.VenueID); id != "" {
		return id, nil
	}
	if strings.TrimSpace(in.Venue) == "" {
		return "", fmt.Errorf("search_openreview_papers: venue 或 venue_id 必填")
	}
	if in.Year <= 0 {
		return "", fmt.Errorf("search_openreview_papers: year 不能为空")
	}
	v := strings.ToLower(strings.TrimSpace(in.Venue))
	switch {
	case v == "neurips" || v == "nips" || strings.Contains(v, "neural information processing systems"):
		return fmt.Sprintf("NeurIPS.cc/%d/Conference", in.Year), nil
	case v == "iclr" || strings.Contains(v, "learning representations"):
		return fmt.Sprintf("ICLR.cc/%d/Conference", in.Year), nil
	case v == "icml" || strings.Contains(v, "machine learning"):
		return fmt.Sprintf("ICML.cc/%d/Conference", in.Year), nil
	default:
		return "", fmt.Errorf("search_openreview_papers: 暂无 %s 的 OpenReview venue 映射,请直接传 venue_id", in.Venue)
	}
}

type openReviewNotesResp struct {
	Notes []openReviewNote `json:"notes"`
}

type openReviewNote struct {
	ID      string                         `json:"id"`
	Forum   string                         `json:"forum"`
	Content map[string]openReviewValueWrap `json:"content"`
}

type openReviewValueWrap struct {
	Value any `json:"value"`
}

func (n openReviewNote) toConferencePaper(venueID string, fallbackYear int) conferencePaper {
	forum := n.Forum
	if forum == "" {
		forum = n.ID
	}
	year := fallbackYear
	if year <= 0 {
		year = yearFromOpenReviewVenueID(venueID)
	}
	pdf := openReviewValueString(n.Content["pdf"])
	return conferencePaper{
		Title:     collapseWS(openReviewValueString(n.Content["title"])),
		Authors:   openReviewValueStrings(n.Content["authors"]),
		Year:      year,
		Venue:     openReviewVenueName(venueID),
		Track:     collapseWS(openReviewValueString(n.Content["venue"])),
		Abstract:  collapseWS(openReviewValueString(n.Content["abstract"])),
		Keywords:  openReviewValueStrings(n.Content["keywords"]),
		URL:       openReviewWebBaseURL + "/forum?id=" + url.QueryEscape(forum),
		PDFURL:    absolutize(openReviewWebBaseURL, pdf),
		Source:    "OpenReview",
		SourceURL: openReviewWebBaseURL + "/group?id=" + url.QueryEscape(venueID),
		BibTeX:    openReviewValueString(n.Content["_bibtex"]),
	}
}

func openReviewValueString(v openReviewValueWrap) string {
	switch x := v.Value.(type) {
	case string:
		return x
	case []any:
		parts := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return ""
	}
}

func openReviewValueStrings(v openReviewValueWrap) []string {
	switch x := v.Value.(type) {
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, collapseWS(s))
			}
		}
		return out
	case string:
		return splitAuthors(x)
	default:
		return nil
	}
}

func yearFromOpenReviewVenueID(venueID string) int {
	for _, part := range strings.Split(venueID, "/") {
		if len(part) == 4 {
			if y, err := strconv.Atoi(part); err == nil && y > 1900 {
				return y
			}
		}
	}
	return 0
}

func openReviewVenueName(venueID string) string {
	if i := strings.Index(venueID, "."); i > 0 {
		return venueID[:i]
	}
	return venueID
}

func filterAndRankConferencePapers(papers []conferencePaper, query string) []conferencePaper {
	q := strings.TrimSpace(query)
	type scored struct {
		p     conferencePaper
		score int
		idx   int
	}
	scoredPapers := make([]scored, 0, len(papers))
	for i, p := range papers {
		score := conferencePaperScore(p, q)
		if q == "" || score > 0 {
			scoredPapers = append(scoredPapers, scored{p: p, score: score, idx: i})
		}
	}
	sort.SliceStable(scoredPapers, func(i, j int) bool {
		if scoredPapers[i].score != scoredPapers[j].score {
			return scoredPapers[i].score > scoredPapers[j].score
		}
		return scoredPapers[i].idx < scoredPapers[j].idx
	})
	out := make([]conferencePaper, 0, len(scoredPapers))
	for _, s := range scoredPapers {
		out = append(out, s.p)
	}
	return out
}

func conferencePaperScore(p conferencePaper, query string) int {
	terms := queryTerms(query)
	if len(terms) == 0 {
		return 1
	}
	title := strings.ToLower(p.Title)
	body := strings.ToLower(strings.Join([]string{
		p.Title,
		strings.Join(p.Authors, " "),
		p.Track,
		p.Abstract,
		strings.Join(p.Keywords, " "),
	}, " "))
	score := 0
	for _, term := range terms {
		switch {
		case strings.Contains(title, term):
			score += 4
		case strings.Contains(body, term):
			score++
		default:
			return 0
		}
	}
	return score
}

func queryTerms(query string) []string {
	stop := map[string]bool{
		"paper": true, "papers": true, "recommend": true, "recommendation": true,
		"related": true, "work": true, "works": true, "conference": true,
		"main": true, "track": true, "year": true,
	}
	seen := map[string]bool{}
	q := strings.ToLower(query)
	for _, zh := range []string{"相关", "推荐", "论文", "有哪些", "会议", "年份", "顶会", "主会"} {
		q = strings.ReplaceAll(q, zh, " ")
	}
	raw := strings.FieldsFunc(q, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		if len(s) <= 1 || stop[s] || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func splitAuthors(s string) []string {
	s = collapseWS(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func absolutize(baseURL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.IsAbs() {
		return u.String()
	}
	base, err := url.Parse(strings.TrimRight(baseURL, "/") + "/")
	if err != nil {
		return raw
	}
	return base.ResolveReference(u).String()
}

func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func classHas(n *html.Node, name string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == name {
			return true
		}
	}
	return false
}

func findDescendant(n *html.Node, match func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}
	if match(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if got := findDescendant(c, match); got != nil {
			return got
		}
	}
	return nil
}

func nodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
			b.WriteByte(' ')
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return collapseWS(b.String())
}
