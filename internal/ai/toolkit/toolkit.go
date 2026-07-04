// Package toolkit 把配置里的工具来源建成 trpc 的工具集与 skill 仓库,按 agent 分组,
// 分别挂到论文助教与小云雀 agent 上。某分组全留空则访问器返回 nil,该 agent 退化纯对话。
package toolkit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/skill"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/mcp"
	tmcp "trpc.group/trpc-go/trpc-mcp-go"

	"GopherPaper/internal/config"
	"GopherPaper/internal/credential"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/constant"
)

// 由 Init 一次性建好,与用户无关。凭据型工具集经 credentialRouter 按 token 隔离:
// streamable transport 会持有 initialize 拿到的 Mcp-Session-Id,共享实例会让不同用户
// 骑同一个远端会话,故每个 token 懒建独立连接,闲置超时回收。
var (
	toolSets   = map[string][]tool.ToolSet{}
	skillRepos = map[string]skill.Repository{}
	// funcTools 按 agent 分组的零散 function tool,如百度地理编码。
	funcTools = map[string][]tool.Tool{}
	// displayNames 工具显示名映射,SSE 推工具状态时把原始工具名换成前端友好名。
	displayNames = maps.Clone(builtinToolDisplayNames)
	// semanticScholarAPIKey 供小云雀工具之外的站内链路复用同一套 S2 配置。
	semanticScholarAPIKey string
)

var builtinToolDisplayNames = map[string]string{
	"current_time":                  "当前时间",
	"delete_my_paper":               "删除我的论文",
	"download_paper":                "下载论文",
	"generate_paper_flow":           "生成思路图",
	"geocode":                       "地点定位",
	"get_paper_citations":           "查被引论文",
	"get_paper_references":          "查参考文献",
	"find_figures":                  "检索论文图表",
	"list_my_papers":                "我的论文列表",
	"recommend_similar_papers":      "相似论文推荐",
	"search_arxiv":                  "arXiv 检索",
	"search_conference_proceedings": "官方会议录检索",
	"search_my_papers":              "检索我的论文",
	"search_openalex":               "OpenAlex 检索",
	"search_paper":                  "检索论文正文",
	"search_sciverse":               "SciVerse 语义检索",
	"read_sciverse_content":         "SciVerse 原文续读",
	"search_openreview_papers":      "OpenReview 检索",
	"search_semantic_scholar":       "学术检索",
	"web_search":                    "联网搜索",
}

// Init 按配置建工具集与 skill 仓库,无配置即空,不报错。
func Init(c config.ToolsConfig) error {
	displayNames = maps.Clone(builtinToolDisplayNames)
	maps.Copy(displayNames, c.ToolNames)
	for _, m := range c.MCP {
		var set tool.ToolSet
		if m.Credential != "" {
			// 凭据型工具集按 token 路由:ctx 无凭据时不碰远端(远端连 initialize 都要
			// 鉴权,裸调只会 401 刷日志),有凭据时每个 token 一条独立连接与远端会话。
			set = newCredentialRouter(m)
		} else {
			set = newMCPSet(m)
		}
		toolSets[m.Agent] = append(toolSets[m.Agent], set)
		zlog.Info("mcp 工具集已登记",
			"name", m.Name, "transport", m.Transport,
			"agent", agentLabel(m.Agent), "credential", m.Credential)
	}
	// 百度地理编码工具给小云雀:把用户口述的地点解析成坐标,补齐瑞幸门店查询的必填经纬度。
	if c.BaiduMapAK != "" {
		funcTools[constant.AgentPioneer] = append(funcTools[constant.AgentPioneer], newGeocodeTool(c.BaiduMapAK, c.BaiduMapSK))
		zlog.Info("geocode 地理编码工具已登记", "agent", constant.AgentPioneer, "sn", c.BaiduMapSK != "")
	}
	// 系统时间工具给小云雀:无外部依赖,无条件登记。
	funcTools[constant.AgentPioneer] = append(funcTools[constant.AgentPioneer], newNowTool())
	// 论文知识库工具给小云雀:列论文(走 paperdao)+ 检索(走 retrieval 检索层),
	// 运行期才触达 DB/Milvus,故无条件登记(此时尚未 Init,但工具只在用户请求时被调用)。
	funcTools[constant.AgentPioneer] = append(funcTools[constant.AgentPioneer],
		newListPapersTool(), newPaperSearchTool(), newDeletePaperTool(), newPaperFlowTool())
	// 论文下载工具给小云雀:把联网找到的论文 PDF 直链下载并导入用户工作台(uploaded,不解析)。
	// 实现由 paperservice.Init 反向注入,破依赖环;此处无条件登记,运行期才触达下载实现。
	funcTools[constant.AgentPioneer] = append(funcTools[constant.AgentPioneer], newDownloadPaperTool())
	// arXiv 学术检索给小云雀:无 key,无条件登记;返回的 pdf_url 与 download_paper 闭环。
	funcTools[constant.AgentPioneer] = append(funcTools[constant.AgentPioneer], newArxivTool())
	// 官方会议论文检索给小云雀:会议+年份类需求先查 proceedings / OpenReview,再用 S2/arXiv 补信息。
	funcTools[constant.AgentPioneer] = append(funcTools[constant.AgentPioneer],
		newConferenceProceedingsTool(),
		newOpenReviewTool(),
	)
	// Semantic Scholar 学术检索给小云雀:key 可选,无条件登记(留空走公共额度);带引用数适合找经典文献。
	// 顺链三件套(相似推荐/被引/参考文献)与 search 共用 key,以 search 结果的 paper_id 顺藤摸瓜。
	// 配了中转代理基址就改走代理(更宽额度 + Bearer 鉴权),绕开官方 search 端点约 1 req/s 的死限。
	if c.SemanticScholarBaseURL != "" {
		setS2Endpoints(c.SemanticScholarBaseURL)
	}
	semanticScholarAPIKey = c.SemanticScholarAPIKey
	funcTools[constant.AgentPioneer] = append(funcTools[constant.AgentPioneer],
		newSemanticScholarTool(c.SemanticScholarAPIKey),
		newS2RecommendTool(c.SemanticScholarAPIKey),
		newS2CitationsTool(c.SemanticScholarAPIKey),
		newS2ReferencesTool(c.SemanticScholarAPIKey),
	)
	zlog.Info("学术检索工具已登记", "agent", constant.AgentPioneer,
		"s2_key", c.SemanticScholarAPIKey != "", "s2_proxy", c.SemanticScholarBaseURL != "")
	// OpenAlex 学术检索给小云雀:覆盖 2.5 亿+ 文献、限流松,作 S2 的冗余/平替。
	// 现按调用计费(免费 key 每天 $1 额度),故配了 key 才登记,留空不裸调。
	if c.OpenAlexAPIKey != "" {
		funcTools[constant.AgentPioneer] = append(funcTools[constant.AgentPioneer], newOpenAlexTool(c.OpenAlexAPIKey))
		zlog.Info("openalex 学术检索工具已登记", "agent", constant.AgentPioneer)
	}
	// SciVerse 语义检索给小云雀:走 /agentic-search 召回正文片段(带页码出处),与按元数据
	// 检索的 OpenAlex/S2 互补。按调用计费且 initialize 即需鉴权,配了 key 才登记,留空不裸调。
	if c.SciVerseAPIKey != "" {
		funcTools[constant.AgentPioneer] = append(funcTools[constant.AgentPioneer],
			newSciVerseTool(c.SciVerseAPIKey),
			newReadSciVerseContentTool(c.SciVerseAPIKey),
		)
		zlog.Info("sciverse 语义检索与续读工具已登记", "agent", constant.AgentPioneer)
	}
	// Tavily 联网搜索给小云雀:补足模型知识截止后的实时信息。
	if c.TavilyAPIKey != "" {
		funcTools[constant.AgentPioneer] = append(funcTools[constant.AgentPioneer], newTavilyTool(c.TavilyAPIKey))
		zlog.Info("tavily 联网搜索工具已登记", "agent", constant.AgentPioneer)
	}
	if err := initSkills(c.Skills, ""); err != nil {
		return err
	}
	if err := initSkills(c.PioneerSkills, constant.AgentPioneer); err != nil {
		return err
	}
	if err := initSkills(c.GopherSkills, constant.AgentGopher); err != nil {
		return err
	}
	return nil
}

// initSkills 给某 agent 分组建 skill 仓库,目录列表为空则跳过。
func initSkills(roots []string, agent string) error {
	if len(roots) == 0 {
		return nil
	}
	repo, err := skill.NewFSRepository(roots...)
	if err != nil {
		return fmt.Errorf("toolkit: 建 %s skill 仓库失败: %w", agentLabel(agent), err)
	}
	skillRepos[agent] = repo
	zlog.Info("skill 仓库已就绪", "agent", agentLabel(agent), "roots", roots)
	return nil
}

// newMCPSet 按配置建一个 mcp 工具集,凭据型挂按请求注入 Authorization 的钩子。
func newMCPSet(m config.MCPServerConfig) tool.ToolSet {
	opts := []mcp.ToolSetOption{mcp.WithName(m.Name)}
	if m.Credential != "" {
		opts = append(opts, mcp.WithMCPOptions(tmcp.WithHTTPBeforeRequest(bearerInjector(m.Credential))))
	}
	return mcp.NewMCPToolSet(mcp.ConnectionConfig{
		Transport: m.Transport,
		ServerURL: m.ServerURL,
		Command:   m.Command,
		Args:      m.Args,
		Headers:   m.Headers,
	}, opts...)
}

// credentialIdleTTL 凭据型连接的闲置回收阈值,超时未用即关闭底层连接与远端会话。
const credentialIdleTTL = 30 * time.Minute

// credentialRouter 按凭据 token 隔离凭据型 mcp 工具集。
// ctx 无凭据时返回空工具不触达远端;有凭据时按 token 摘要懒建独立 toolset,
// 各 token 各持一条连接与 Mcp-Session-Id,远端会话天然按用户隔离,换绑 token 即换新会话。
// 需配合 agent 的运行期工具刷新,让每轮用请求 ctx 重新求值。
type credentialRouter struct {
	cfg      config.MCPServerConfig
	provider string

	mu   sync.Mutex
	sets map[string]*routedSet // token 摘要 -> 该 token 专属工具集
}

type routedSet struct {
	set      tool.ToolSet
	lastUsed time.Time
}

func newCredentialRouter(cfg config.MCPServerConfig) *credentialRouter {
	return &credentialRouter{cfg: cfg, provider: cfg.Credential, sets: map[string]*routedSet{}}
}

func (r *credentialRouter) Name() string { return r.cfg.Name }

func (r *credentialRouter) Tools(ctx context.Context) []tool.Tool {
	tok := credential.From(ctx, r.provider)
	if tok == "" {
		return nil
	}
	return r.setFor(tok).Tools(ctx)
}

// setFor 取该 token 的专属工具集,缺失则懒建;顺路回收闲置超时的其他 token 连接。
func (r *credentialRouter) setFor(tok string) tool.ToolSet {
	sum := sha256.Sum256([]byte(tok))
	key := hex.EncodeToString(sum[:8])
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()
	for k, e := range r.sets {
		if k != key && now.Sub(e.lastUsed) > credentialIdleTTL {
			go func(s tool.ToolSet) { _ = s.Close() }(e.set)
			delete(r.sets, k)
		}
	}
	if e, ok := r.sets[key]; ok {
		e.lastUsed = now
		return e.set
	}
	e := &routedSet{set: newMCPSet(r.cfg), lastUsed: now}
	r.sets[key] = e
	zlog.Info("凭据型 mcp 连接已建立", "name", r.cfg.Name, "provider", r.provider, "active", len(r.sets))
	return e.set
}

func (r *credentialRouter) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, e := range r.sets {
		_ = e.set.Close()
		delete(r.sets, k)
	}
	return nil
}

// bearerInjector 生成按请求注入 Authorization 的钩子:每次工具调用时从 ctx 取该
// provider 的凭据写 Bearer 头;ctx 无凭据则不动请求,由远端 401 触发 agent 引导绑定。
func bearerInjector(provider string) tmcp.HTTPBeforeRequestFunc {
	return func(ctx context.Context, req *http.Request) error {
		if tok := credential.From(ctx, provider); tok != "" {
			req.Header.Set(constant.HeaderAuthorization, "Bearer "+tok)
		}
		return nil
	}
}

// agentLabel 把空分组名映射为默认助教的可读名,只用于日志。
func agentLabel(agent string) string {
	if agent == "" {
		return "chat"
	}
	return agent
}

// DisplayName 把原始工具名换成配置的前端显示名,未配置回退原始名。
func DisplayName(name string) string {
	if d, ok := displayNames[name]; ok && d != "" {
		return d
	}
	return name
}

func CapabilityInstruction(agent string) string {
	if agent != constant.AgentPioneer {
		return ""
	}
	names := functionToolNames(agent)
	missing := []string{}
	for _, name := range []string{"search_openalex", "search_sciverse", "read_sciverse_content", "web_search"} {
		if !names[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return ""
	}
	sort.Strings(missing)
	var b strings.Builder
	b.WriteString("当前运行环境没有启用以下工具:")
	b.WriteString(strings.Join(missing, "、"))
	b.WriteString("。不得调用或声称已调用这些工具;遇到原规则要求使用但当前未启用的工具时,必须降级到已启用的替代工具并向用户简要说明限制。")
	if !names["search_sciverse"] {
		b.WriteString(" 综述、趋势、进展类问题不再强制使用 search_sciverse,应改用 search_semantic_scholar、search_arxiv、search_conference_proceedings、search_openreview_papers")
		if names["search_openalex"] {
			b.WriteString("、search_openalex")
		}
		b.WriteString(" 等已启用来源交叉核对;没有片段级正文证据时,明确标注结论基于题录/摘要元数据而非原文。")
	}
	if !names["search_openalex"] {
		b.WriteString(" OpenAlex 未启用时,不要把它作为 Semantic Scholar 的兜底;可改用 arXiv、官方会议源或 SciVerse(若可用)。")
	}
	if !names["web_search"] {
		b.WriteString(" web_search 未启用时,实时网页信息无法联网补充,不要编造网页结果。")
	}
	return b.String()
}

func functionToolNames(agent string) map[string]bool {
	out := map[string]bool{}
	for _, t := range funcTools[agent] {
		if d := t.Declaration(); d != nil && d.Name != "" {
			out[d.Name] = true
		}
	}
	return out
}

// ToolSets 返回默认论文助教的 mcp 工具集,未配置返回 nil。
func ToolSets() []tool.ToolSet { return toolSets[""] }

// ToolSetsFor 返回指定 agent 分组的 mcp 工具集,未配置返回 nil。
func ToolSetsFor(agent string) []tool.ToolSet { return toolSets[agent] }

// ToolsFor 返回指定 agent 分组的 function tool,未配置返回 nil。
func ToolsFor(agent string) []tool.Tool { return funcTools[agent] }

// SkillRepo 返回默认论文助教的 skill 仓库,未配置返回 nil。
func SkillRepo() skill.Repository { return skillRepos[""] }

// SkillRepoFor 返回指定 agent 分组的 skill 仓库,未配置返回 nil。
func SkillRepoFor(agent string) skill.Repository { return skillRepos[agent] }
