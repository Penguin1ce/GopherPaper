package toolkit

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/tool"

	"GopherPaper/internal/ai/ragtools"
	"GopherPaper/internal/config"
	"GopherPaper/internal/credential"
	"GopherPaper/pkg/constant"
)

// TestInitGroupsByAgent 验证 mcp 工具集按 agent 字段分组,互不可见。
func TestInitGroupsByAgent(t *testing.T) {
	toolSets = map[string][]tool.ToolSet{}
	if err := Init(config.ToolsConfig{MCP: []config.MCPServerConfig{
		{Name: "paper-tool", Transport: "streamable", ServerURL: "http://chat.test/mcp"},
		{Name: "my-coffee", Transport: "streamable", ServerURL: "http://luckin.test/mcp", Agent: constant.AgentPioneer, Credential: constant.CredentialLuckin},
	}}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if n := len(ToolSets()); n != 1 {
		t.Fatalf("默认分组应有 1 个工具集,得到 %d", n)
	}
	if n := len(ToolSetsFor(constant.AgentPioneer)); n != 1 {
		t.Fatalf("pioneer 分组应有 1 个工具集,得到 %d", n)
	}
}

func TestBuiltinFunctionToolsHaveDisplayNames(t *testing.T) {
	oldToolSets, oldFuncTools, oldDisplayNames := toolSets, funcTools, displayNames
	defer func() {
		toolSets = oldToolSets
		funcTools = oldFuncTools
		displayNames = oldDisplayNames
	}()
	toolSets = map[string][]tool.ToolSet{}
	funcTools = map[string][]tool.Tool{}

	if err := Init(config.ToolsConfig{
		BaiduMapAK:            "baidu-ak",
		TavilyAPIKey:          "tavily-key",
		SemanticScholarAPIKey: "s2-key",
		OpenAlexAPIKey:        "openalex-key",
		SciVerseAPIKey:        "sciverse-key",
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	for _, tl := range ToolsFor(constant.AgentPioneer) {
		decl := tl.Declaration()
		if decl == nil || decl.Name == "" {
			t.Fatalf("工具声明缺少名称: %#v", decl)
		}
		if got := DisplayName(decl.Name); got == decl.Name {
			t.Errorf("工具 %s 缺少中文显示名", decl.Name)
		}
	}
}

func TestRAGFunctionToolsHaveDisplayNames(t *testing.T) {
	oldDisplayNames := displayNames
	defer func() { displayNames = oldDisplayNames }()
	displayNames = maps.Clone(builtinToolDisplayNames)
	for _, tl := range ragtools.All() {
		decl := tl.Declaration()
		if decl == nil || decl.Name == "" {
			t.Fatalf("工具声明缺少名称: %#v", decl)
		}
		if got := DisplayName(decl.Name); got == decl.Name {
			t.Errorf("工具 %s 缺少中文显示名", decl.Name)
		}
	}
}

func TestDisplayNameConfigOverridesBuiltin(t *testing.T) {
	oldToolSets, oldFuncTools, oldDisplayNames := toolSets, funcTools, displayNames
	defer func() {
		toolSets = oldToolSets
		funcTools = oldFuncTools
		displayNames = oldDisplayNames
	}()
	toolSets = map[string][]tool.ToolSet{}
	funcTools = map[string][]tool.Tool{}

	if err := Init(config.ToolsConfig{
		ToolNames: map[string]string{"current_time": "取当前时间"},
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got := DisplayName("current_time"); got != "取当前时间" {
		t.Fatalf("配置应覆盖内置显示名,得到 %q", got)
	}
	if got := DisplayName("unknown_tool"); got != "unknown_tool" {
		t.Fatalf("未知工具应回退原名,得到 %q", got)
	}
}

// TestGeocode 验证百度地理编码的请求参数与响应解析,status 非 0 报错。
func TestGeocode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("ak"); got != "test-ak" {
			t.Errorf("ak = %q", got)
		}
		if got := r.URL.Query().Get("ret_coordtype"); got != "gcj02ll" {
			t.Errorf("ret_coordtype = %q", got)
		}
		switch r.URL.Query().Get("address") {
		case "北京市朝阳区国贸":
			fmt.Fprint(w, `{"status":0,"result":{"location":{"lng":116.46,"lat":39.91},"confidence":80,"level":"商务大厦"}}`)
		default:
			fmt.Fprint(w, `{"status":1,"message":"内部错误"}`)
		}
	}))
	defer srv.Close()
	old := geocodeBaseURL
	geocodeBaseURL = srv.URL
	defer func() { geocodeBaseURL = old }()

	out, err := geocode(context.Background(), "test-ak", "", geocodeInput{Address: "北京市朝阳区国贸"})
	if err != nil {
		t.Fatalf("geocode: %v", err)
	}
	if out.Longitude != 116.46 || out.Latitude != 39.91 || out.Level != "商务大厦" {
		t.Fatalf("解析结果不符: %+v", out)
	}

	if _, err := geocode(context.Background(), "test-ak", "", geocodeInput{Address: "不存在的地方"}); err == nil {
		t.Fatal("status 非 0 应报错")
	}
	if _, err := geocode(context.Background(), "test-ak", "", geocodeInput{}); err == nil {
		t.Fatal("空地址应报错")
	}
}

// TestCredentialRouter 验证 ctx 无凭据时返回空且不建连接;有凭据时按 token 隔离实例,
// 同 token 复用、异 token 独立,闲置超时的连接在后续访问时被回收。
func TestCredentialRouter(t *testing.T) {
	r := newCredentialRouter(config.MCPServerConfig{
		Name:       "my-coffee",
		Transport:  "streamable",
		ServerURL:  "http://luckin.test/mcp",
		Credential: constant.CredentialLuckin,
	})
	if got := r.Tools(context.Background()); len(got) != 0 {
		t.Fatalf("无凭据时应返回空,得到 %d 个", len(got))
	}
	if len(r.sets) != 0 {
		t.Fatalf("无凭据时不应建连接,得到 %d 条", len(r.sets))
	}

	a := r.setFor("tok-a")
	if r.setFor("tok-a") != a {
		t.Fatal("同 token 应复用同一工具集")
	}
	if r.setFor("tok-b") == a {
		t.Fatal("不同 token 应各持独立工具集")
	}
	if len(r.sets) != 2 {
		t.Fatalf("应有 2 条连接,得到 %d", len(r.sets))
	}

	// 把 tok-a 标成闲置超时,下次访问其他 token 时应被回收。
	for _, e := range r.sets {
		if e.set == a {
			e.lastUsed = time.Now().Add(-2 * credentialIdleTTL)
		}
	}
	r.setFor("tok-b")
	if len(r.sets) != 1 {
		t.Fatalf("闲置连接应被回收,剩 %d 条", len(r.sets))
	}
}

// TestBearerInjector 验证钩子从 ctx 取对应 provider 的凭据写 Bearer 头,无凭据不动请求。
func TestBearerInjector(t *testing.T) {
	hook := bearerInjector(constant.CredentialLuckin)

	req, _ := http.NewRequest(http.MethodPost, "http://luckin.test/mcp", nil)
	ctx := credential.With(context.Background(), constant.CredentialLuckin, "tok-123")
	if err := hook(ctx, req); err != nil {
		t.Fatalf("hook: %v", err)
	}
	if got := req.Header.Get(constant.HeaderAuthorization); got != "Bearer tok-123" {
		t.Fatalf("Authorization = %q", got)
	}

	req2, _ := http.NewRequest(http.MethodPost, "http://luckin.test/mcp", nil)
	if err := hook(context.Background(), req2); err != nil {
		t.Fatalf("hook: %v", err)
	}
	if got := req2.Header.Get(constant.HeaderAuthorization); got != "" {
		t.Fatalf("无凭据时不应写 Authorization,得到 %q", got)
	}

	req3, _ := http.NewRequest(http.MethodPost, "http://luckin.test/mcp", nil)
	other := credential.With(context.Background(), "other-provider", "tok-456")
	if err := hook(other, req3); err != nil {
		t.Fatalf("hook: %v", err)
	}
	if got := req3.Header.Get(constant.HeaderAuthorization); got != "" {
		t.Fatalf("其他 provider 的凭据不应被注入,得到 %q", got)
	}
}
