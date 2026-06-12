// Package toolkit 把配置里的工具来源建成 trpc 的工具集与 skill 仓库,按 agent 分组,
// 分别挂到论文助教与先锋者 agent 上。某分组全留空则访问器返回 nil,该 agent 退化纯对话。
package toolkit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"net/http"
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
	displayNames = map[string]string{}
)

// Init 按配置建工具集与 skill 仓库,无配置即空,不报错。
func Init(c config.ToolsConfig) error {
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
	// 百度地理编码工具给先锋者:把用户口述的地点解析成坐标,补齐瑞幸门店查询的必填经纬度。
	if c.BaiduMapAK != "" {
		funcTools[constant.AgentPioneer] = append(funcTools[constant.AgentPioneer], newGeocodeTool(c.BaiduMapAK, c.BaiduMapSK))
		zlog.Info("geocode 地理编码工具已登记", "agent", constant.AgentPioneer, "sn", c.BaiduMapSK != "")
	}
	if err := initSkills(c.Skills, ""); err != nil {
		return err
	}
	return initSkills(c.PioneerSkills, constant.AgentPioneer)
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
