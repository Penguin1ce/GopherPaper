// toolkit semanticscholar_related.go 在 search 之外补 Semantic Scholar 的"顺藤摸瓜"能力:
// 给定一篇论文,找引用它的后续工作(get_paper_citations)、它引用的奠基文献(get_paper_references)、
// 以及内容相近的推荐论文(recommend_similar_papers)。三者输入都是论文标识(S2 paperId、arXiv
// 编号或 DOI),输出与 search_semantic_scholar 同构(带 paper_id/pdf_url/BibTeX),可继续转交
// download_paper 下载入库,或把返回的 paper_id 再喂回这三个工具继续顺链。
//
// 复用 semanticscholar.go 的 s2Get(统一退避重试与 key 注入)、s2PaperRec(记录转换)、s2Fields
// (请求字段集)与 apiKey 约定;key 留空走公共额度(限流紧),配置后走专属额度。
package toolkit

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// S2 端点基址,测试可替换。citations/references 拼在 paper/{id} 之后,推荐走独立的 recommendations 服务。
var (
	s2PaperBaseURL     = "https://api.semanticscholar.org/graph/v1/paper"
	s2RecommendBaseURL = "https://api.semanticscholar.org/recommendations/v1/papers/forpaper"
)

// s2ArxivIDRe 粗匹配新式 arXiv 编号,第一组捕获去版本后的主体,第二组吃掉可选版本号。
var s2ArxivIDRe = regexp.MustCompile(`^(\d{4}\.\d{4,5})(v\d+)?$`)

// s2UUIDRe 匹配本站论文 UUID(上传/解析产生),模型常把它误当 S2 标识传进来。
var s2UUIDRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// normalizeS2PaperID 把模型给的论文标识规整成 S2 接受的 ID 形式:
// 已带前缀(含冒号,如 ARXIV:/DOI:/CorpusId:)或裸 S2 paperId 原样返回;
// 裸 arXiv 编号去版本号后补 ARXIV:(S2 的 ARXIV: 标识不认 v1/v2 后缀,带上必 404),
// 10. 开头的裸 DOI 补 DOI:。
func normalizeS2PaperID(raw string) string {
	id := strings.TrimSpace(raw)
	if id == "" || strings.Contains(id, ":") {
		return id
	}
	if m := s2ArxivIDRe.FindStringSubmatch(id); m != nil {
		return "ARXIV:" + m[1]
	}
	if strings.HasPrefix(id, "10.") {
		return "DOI:" + id
	}
	return id
}

// s2RelatedInput 是三个顺链工具的公共入参:一个论文标识 + 条数与可下载过滤。
type s2RelatedInput struct {
	PaperID        string `json:"paper_id" jsonschema:"description=目标论文标识:优先用 search_semantic_scholar 结果里的 paper_id;也接受 arXiv 编号(如 2106.15928)或 DOI(如 10.18653/v1/2021.emnlp),required"`
	MaxResults     int    `json:"max_results,omitempty" jsonschema:"description=返回条数,默认 10,上限 20"`
	OpenAccessOnly bool   `json:"open_access_only,omitempty" jsonschema:"description=只返回有可下载 pdf_url(含 arXiv 镜像兜底)的论文;仅当只要能下载/导入的论文时才设 true,默认 false 以免漏掉无开放全文的顶会论文"`
}

// s2CitationsResp /citations 响应:每条把论文嵌在 citingPaper 下。
type s2CitationsResp struct {
	Data []struct {
		CitingPaper s2PaperRec `json:"citingPaper"`
	} `json:"data"`
}

// s2ReferencesResp /references 响应:每条把论文嵌在 citedPaper 下。
type s2ReferencesResp struct {
	Data []struct {
		CitedPaper s2PaperRec `json:"citedPaper"`
	} `json:"data"`
}

// s2RecommendResp 推荐服务响应:论文列表在 recommendedPapers 下。
type s2RecommendResp struct {
	RecommendedPapers []s2PaperRec `json:"recommendedPapers"`
}

func newS2CitationsTool(apiKey string) tool.Tool {
	fn := func(ctx context.Context, in s2RelatedInput) (s2Output, error) {
		return s2Citations(ctx, apiKey, in)
	}
	return function.NewFunctionTool(fn,
		function.WithName("get_paper_citations"),
		function.WithDescription("查某篇论文被哪些后续论文引用(顺着引用往更新的工作走),输入 paper_id(search_semantic_scholar 结果里的 paper_id,或 arXiv 编号/DOI)。返回与 search 同构,带 paper_id/被引数/pdf_url/BibTeX,可继续转交 download_paper 或再查其引用。想了解一篇论文的后续影响、找最新跟进工作、补全相关工作里的「后来者」时用。"),
	)
}

func newS2ReferencesTool(apiKey string) tool.Tool {
	fn := func(ctx context.Context, in s2RelatedInput) (s2Output, error) {
		return s2References(ctx, apiKey, in)
	}
	return function.NewFunctionTool(fn,
		function.WithName("get_paper_references"),
		function.WithDescription("查某篇论文引用了哪些文献(往奠基性工作回溯),输入 paper_id(search_semantic_scholar 结果里的 paper_id,或 arXiv 编号/DOI)。返回与 search 同构,带 paper_id/被引数/pdf_url/BibTeX,可继续转交 download_paper 或再查其参考文献。想顺着一篇论文找它依赖的经典方法、补全相关工作里的「前置工作」时用。"),
	)
}

func newS2RecommendTool(apiKey string) tool.Tool {
	fn := func(ctx context.Context, in s2RelatedInput) (s2Output, error) {
		return s2Recommend(ctx, apiKey, in)
	}
	return function.NewFunctionTool(fn,
		function.WithName("recommend_similar_papers"),
		function.WithDescription("给定一篇论文找内容相近的推荐论文(Semantic Scholar 推荐服务),输入 paper_id(search_semantic_scholar 结果里的 paper_id,或 arXiv 编号/DOI)。返回与 search 同构,带 paper_id/被引数/pdf_url/BibTeX。写相关工作/文献综述、围绕一篇核心论文扩展阅读面时用——比关键词检索更贴近这篇论文的主题。"),
	)
}

func s2Citations(ctx context.Context, apiKey string, in s2RelatedInput) (s2Output, error) {
	id, n, limit, err := s2RelatedReq("get_paper_citations", in)
	if err != nil {
		return s2Output{}, err
	}
	reqURL := s2PaperBaseURL + "/" + url.PathEscape(id) + "/citations?" + s2RelatedParams(limit)
	var r s2CitationsResp
	if err := s2Get(ctx, apiKey, reqURL, &r); err != nil {
		return s2Output{}, wrapS2RelatedErr("get_paper_citations", err)
	}
	recs := make([]s2PaperRec, 0, len(r.Data))
	for _, d := range r.Data {
		recs = append(recs, d.CitingPaper)
	}
	return s2RelatedPapers(recs, n, in.OpenAccessOnly), nil
}

func s2References(ctx context.Context, apiKey string, in s2RelatedInput) (s2Output, error) {
	id, n, limit, err := s2RelatedReq("get_paper_references", in)
	if err != nil {
		return s2Output{}, err
	}
	reqURL := s2PaperBaseURL + "/" + url.PathEscape(id) + "/references?" + s2RelatedParams(limit)
	var r s2ReferencesResp
	if err := s2Get(ctx, apiKey, reqURL, &r); err != nil {
		return s2Output{}, wrapS2RelatedErr("get_paper_references", err)
	}
	recs := make([]s2PaperRec, 0, len(r.Data))
	for _, d := range r.Data {
		recs = append(recs, d.CitedPaper)
	}
	return s2RelatedPapers(recs, n, in.OpenAccessOnly), nil
}

func s2Recommend(ctx context.Context, apiKey string, in s2RelatedInput) (s2Output, error) {
	id, n, limit, err := s2RelatedReq("recommend_similar_papers", in)
	if err != nil {
		return s2Output{}, err
	}
	reqURL := s2RecommendBaseURL + "/" + url.PathEscape(id) + "?" + s2RelatedParams(limit)
	var r s2RecommendResp
	if err := s2Get(ctx, apiKey, reqURL, &r); err != nil {
		return s2Output{}, wrapS2RelatedErr("recommend_similar_papers", err)
	}
	return s2RelatedPapers(r.RecommendedPapers, n, in.OpenAccessOnly), nil
}

// s2RelatedReq 校验入参并算出规整后的论文 ID、目标条数 n 与请求 limit
// (要按可下载过滤时多取候选,过滤会砍掉一部分)。
func s2RelatedReq(toolName string, in s2RelatedInput) (id string, n, limit int, err error) {
	raw := strings.TrimSpace(in.PaperID)
	if raw == "" {
		return "", 0, 0, fmt.Errorf("%s: paper_id 不能为空", toolName)
	}
	// 站内论文 UUID 不是 S2 标识,提前点破并引导,免得撞 404 后模型反复重试。
	if s2UUIDRe.MatchString(raw) {
		return "", 0, 0, fmt.Errorf("%s: paper_id %q 是本站论文 ID,不是 Semantic Scholar 标识。请先用 search_semantic_scholar 按论文标题检索拿到 paper_id,或改用 arXiv 编号/DOI", toolName, raw)
	}
	id = normalizeS2PaperID(raw)
	n = clampResults(in.MaxResults, 10, 20)
	limit = n
	if in.OpenAccessOnly {
		limit = min(n*4, 100) // S2 单页上限 100
	}
	return id, n, limit, nil
}

// s2RelatedParams 拼三个顺链端点共用的查询串:论文字段集 + 条数。
func s2RelatedParams(limit int) string {
	params := url.Values{}
	params.Set("fields", s2Fields)
	params.Set("limit", fmt.Sprint(limit))
	return params.Encode()
}

// s2RelatedPapers 把原始记录转成对外列表:跳过空标题,可选只留可下载,截断到 n。
func s2RelatedPapers(recs []s2PaperRec, n int, openOnly bool) s2Output {
	papers := make([]s2Paper, 0, len(recs))
	for _, d := range recs {
		if strings.TrimSpace(d.Title) == "" {
			continue
		}
		p := d.toPaper()
		if openOnly && p.PDFURL == "" {
			continue
		}
		papers = append(papers, p)
		if len(papers) >= n {
			break
		}
	}
	return s2Output{Papers: papers}
}

// wrapS2RelatedErr 给顺链工具的错误带上工具名;404 译成可读提示。
func wrapS2RelatedErr(toolName string, err error) error {
	if errors.Is(err, errS2NotFound) {
		return fmt.Errorf("%s: 未找到该 paper_id 对应的论文,确认 paper_id 来自 search_semantic_scholar 结果,或换用 arXiv 编号/DOI", toolName)
	}
	return fmt.Errorf("%s: %w", toolName, err)
}
