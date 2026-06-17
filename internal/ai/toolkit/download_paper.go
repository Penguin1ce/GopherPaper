// toolkit download_paper.go 是小云雀的论文下载工具:把用户在学术站找到的论文 PDF 直链下载并导入工作台,
// 随即自动进解析流水线(MinerU 解析→抽取→入库),用户稍后在工作台查看进度与研读。
//
// 安全限制:只允许下载白名单内正规学术开放获取站点的 PDF(host 白名单 + 重定向逐跳校验),
// 既贴合论文场景,也把"服务端按任意 URL 发请求"的 SSRF 面收敛到一批可信域(配合 search_arxiv /
// search_semantic_scholar 给出的来源)。白名单每加一个域都要重新评估;若日后改为开放任意域,
// 必须改补「解析 host→IP 拒绝私有/环回/链路本地段」的 IP 层防护。
//
// 下载逻辑(HTTP 拉取、域名校验、PDF 校验、文件名推断)属工具职责,全在本包;落盘建记录并投解析队列
// 要碰 dao/model/mq,由 service/paper 经 RegisterPaperIngest 注入(service/paper→ai→agentrt→toolkit
// 已单向,toolkit 直接 import 会成环,故按 no-DI 惯例反向注入破环)。
package toolkit

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"

	"GopherPaper/pkg/constant"
)

// downloadClient 下载论文 PDF 的 HTTP 客户端,留足超时容纳几十 MB 的大文件。
// 重定向逐跳校验:学术站的 /pdf/ 链接常 301 到带 .pdf 的地址(或开放库的 CDN),
// 但任何跳出白名单域的重定向一律拒绝,防被 30x 跳到内网绕过对初始 URL 的校验。
var downloadClient = &http.Client{
	Timeout: 90 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("download_paper: 重定向过多")
		}
		if !hostAllowed(req.URL.Hostname()) {
			return fmt.Errorf("download_paper: 重定向目标 %s 不在允许域内", req.URL.Hostname())
		}
		return nil
	},
}

// allowedPaperHosts 是论文下载的域名白名单:一批正规学术开放获取站点的注册域。
// 命中规则为精确匹配或子域后缀匹配(host==d 或 host 以 "."+d 结尾),
// 故 proceedings.mlr.press 命中 mlr.press、pdfs.semanticscholar.org 命中 semanticscholar.org。
// 这些站均以开放获取直链 PDF 为主,贴合论文场景且把 SSRF 面收敛到可信域。
var allowedPaperHosts = []string{
	"arxiv.org",          // arXiv 预印本
	"biorxiv.org",        // bioRxiv 生物学预印本
	"medrxiv.org",        // medRxiv 医学预印本
	"openreview.net",     // OpenReview 审稿平台
	"aclanthology.org",   // ACL Anthology 计算语言学
	"ncbi.nlm.nih.gov",   // PubMed Central 开放全文
	"mlr.press",          // PMLR 机器学习会议录
	"nips.cc",            // NeurIPS 早期会议录(papers.nips.cc)
	"neurips.cc",         // NeurIPS 会议录(proceedings.neurips.cc)
	"semanticscholar.org", // Semantic Scholar 开放 PDF 镜像
	"thecvf.com",         // CVF 开放获取(CVPR/ICCV 等)
	"ieeexplore.ieee.org", // IEEE Xplore(多为订阅站,直链 PDF 需机构权限才下得到)
}

// hostAllowed 判断下载域名是否在学术站白名单内:精确或子域后缀匹配。
func hostAllowed(host string) bool {
	host = strings.ToLower(host)
	for _, d := range allowedPaperHosts {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// IngestedPaper 是下载导入的结果,字段为基本类型以免 toolkit 反向耦合 service 的 model。
type IngestedPaper struct {
	PaperID string
	Title   string
	Status  string
}

// ingestPaper 由 service/paper 注入:把下载到的 PDF 字节落盘建 Paper 记录并投解析队列。
// owner 在实现侧从 ctx 的 tenant 取,故签名只需文件名与字节。
var ingestPaper func(ctx context.Context, fileName string, data []byte) (IngestedPaper, error)

// RegisterPaperIngest 注册论文入库实现,破 toolkit↔paperservice 依赖环。
func RegisterPaperIngest(fn func(ctx context.Context, fileName string, data []byte) (IngestedPaper, error)) {
	ingestPaper = fn
}

type downloadPaperInput struct {
	URL   string `json:"url" jsonschema:"description=论文的 PDF 直链(如 https://arxiv.org/pdf/2503.03480),须是 PDF 直链不能是摘要页;支持 arxiv/bioRxiv/medRxiv/OpenReview/ACL Anthology/PMC/PMLR/NeurIPS/CVF/Semantic Scholar 等正规学术开放站,其他站点会被拒绝,required"`
	Title string `json:"title,omitempty" jsonschema:"description=论文标题,留空则用链接推断的文件名"`
}

type downloadPaperOutput struct {
	PaperID string `json:"paper_id" jsonschema:"description=导入工作台后的论文 id"`
	Title   string `json:"title" jsonschema:"description=论文标题"`
	Status  string `json:"status" jsonschema:"description=论文状态,uploaded 表示已入库待解析,随后自动进入解析流程"`
	Message string `json:"message" jsonschema:"description=给用户的提示,告知已导入工作台并开始自动解析"`
}

// newDownloadPaperTool 构建论文下载工具:本包下载白名单学术站的 PDF,再经注入的 ingestPaper 入库到本人工作台并解析。
func newDownloadPaperTool() tool.Tool {
	fn := func(ctx context.Context, in downloadPaperInput) (downloadPaperOutput, error) {
		if ingestPaper == nil {
			return downloadPaperOutput{}, fmt.Errorf("download_paper: 论文入库能力未就绪")
		}
		rawURL := strings.TrimSpace(in.URL)
		if rawURL == "" {
			return downloadPaperOutput{}, fmt.Errorf("download_paper: url 不能为空")
		}
		data, err := fetchPDF(ctx, rawURL)
		if err != nil {
			return downloadPaperOutput{}, err
		}
		p, err := ingestPaper(ctx, fileNameFor(rawURL, strings.TrimSpace(in.Title)), data)
		if err != nil {
			return downloadPaperOutput{}, fmt.Errorf("download_paper: 导入工作台失败: %w", err)
		}
		return downloadPaperOutput{
			PaperID: p.PaperID,
			Title:   p.Title,
			Status:  p.Status,
			Message: "已下载并导入工作台,正在自动解析,稍后可在工作台查看进度与研读。",
		}, nil
	}
	return function.NewFunctionTool(fn,
		function.WithName("download_paper"),
		function.WithDescription("把一篇论文的 PDF 下载并导入用户的工作台,导入后自动解析入库。须传 PDF 直链(如 https://arxiv.org/pdf/2503.03480),不要传摘要页;支持 arxiv、bioRxiv、medRxiv、OpenReview、ACL Anthology、PMC、PMLR、NeurIPS、CVF、Semantic Scholar 等正规学术开放站,可直接用 search_arxiv / search_semantic_scholar 返回的 pdf_url。"),
	)
}

// fetchPDF 下载 URL 内容,校验域名在白名单内、是 PDF(magic 头)且不超体积上限,返回字节。
func fetchPDF(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("download_paper: 无效的下载链接: %s", rawURL)
	}
	if !hostAllowed(u.Hostname()) {
		return nil, fmt.Errorf("download_paper: 域名 %s 不在支持的学术站白名单内,请换 arxiv/bioRxiv/OpenReview/ACL/PMC/PMLR/NeurIPS/CVF/Semantic Scholar 等开放站的 PDF 直链", u.Hostname())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("download_paper: 构建下载请求失败: %w", err)
	}
	setAcademicHeaders(req)
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download_paper: 下载论文失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download_paper: 下载论文返回 %d", resp.StatusCode)
	}
	// 多读 1 字节用于判断是否超限;magic 校验拦住摘要页等非 PDF 内容。
	data, err := io.ReadAll(io.LimitReader(resp.Body, constant.MaxPaperDownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("download_paper: 读取下载内容失败: %w", err)
	}
	if len(data) > constant.MaxPaperDownloadBytes {
		return nil, fmt.Errorf("download_paper: 论文超过 %dMB 体积上限", constant.MaxPaperDownloadBytes>>20)
	}
	if !strings.HasPrefix(string(data), "%PDF") {
		return nil, fmt.Errorf("download_paper: 链接不是可直接下载的 PDF(可能指向 HTML 摘要页),请换 arxiv 的 /pdf/ 直链")
	}
	return data, nil
}

// fileNameFor 给下载的论文定文件名:优先用 title,否则取 URL 路径末段,兜底 paper;统一补 .pdf。
func fileNameFor(rawURL, title string) string {
	name := strings.TrimSpace(title)
	if name == "" {
		if u, err := url.Parse(rawURL); err == nil {
			base := path.Base(u.Path)
			// 只在确实是 .pdf 后缀时去掉,避免把 arxiv 编号 2503.03480 的 .03480 误当扩展名删掉
			if strings.EqualFold(path.Ext(base), ".pdf") {
				base = strings.TrimSuffix(base, path.Ext(base))
			}
			name = base
		}
	}
	if name == "" {
		name = "paper"
	}
	return name + ".pdf"
}
