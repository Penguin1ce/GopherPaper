package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	trpcmodel "trpc.group/trpc-go/trpc-agent-go/model"

	"GopherPaper/internal/ai/core"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/config"
	"GopherPaper/internal/tenant"
	"GopherPaper/pkg/constant"
)

func loadChatTestModels(t *testing.T) (*config.Config, bool) {
	t.Helper()
	cfg, err := config.Load("../../../config/config.toml")
	if err != nil {
		t.Skipf("跳过:未找到可用 config.toml: %v", err)
	}
	if cfg.Models.Intent.APIKey == "" || cfg.Models.Chat.APIKey == "" {
		t.Skip("跳过:模型未配置网关或密钥")
	}
	return cfg, true
}

// TestClassifyIntent 验证意图分类链路把各类 query 路由到期望子类。
func TestClassifyIntent(t *testing.T) {
	cfg, _ := loadChatTestModels(t)
	aimodel.Init(cfg) // 意图模型按 ctx 的 tenant 自取,须先注入配置

	cases := map[string]constant.IntentType{
		"你好，辛苦啦": constant.IntentChitchat,
		"这篇论文在 DBLP 数据集上的准确率是多少？": constant.IntentSummary,
		"帮我概括一下这篇论文讲了什么":          constant.IntentSummary,
		"它用了什么模型结构和实验设计？":         constant.IntentMethod,
	}

	for q, want := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		ctx = tenant.With(ctx, tenant.Tenant{StudentID: "test-user"})
		got := ClassifyIntent(ctx, q)
		cancel()
		if got != want {
			t.Errorf("意图分类不符: %q 期望 %q 实得 %q", q, want, got)
		}
	}
}

// TestChat 验证 chat 切片端到端:分类 + RAG 生成 + 意图标记。
// 检索器未 Init 时降级为无片段(像 report 切片),仍能生成,故不依赖 Milvus。
func TestChat(t *testing.T) {
	cfg, _ := loadChatTestModels(t)
	aimodel.Init(cfg) // 模型经 ctx 的 tenant 按用户取,须先注入配置

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ctx = tenant.With(ctx, tenant.Tenant{StudentID: "test-user"})

	reply, err := Chat(ctx, nil, "请简要介绍这篇论文的研究方法")
	if err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}
	if strings.TrimSpace(reply.Content) == "" {
		t.Fatal("回答内容为空")
	}
	if !reply.Intent.IsChat() {
		t.Fatalf("回答意图非法: %q", reply.Intent)
	}
	t.Logf("trpc chat 切片跑通: intent=%q 内容长度=%d", reply.Intent, len(reply.Content))
}

func TestParseIntent(t *testing.T) {
	cases := map[string]constant.IntentType{
		`{"type":"chitchat"}`: constant.IntentChitchat,
		`{"type":"summary"}`:  constant.IntentSummary,
		`{"type":"method"}`:   constant.IntentMethod,
		`{"type":"fact"}`:     constant.IntentSummary,
		"闲聊":                  constant.IntentChitchat,
		"fact":                constant.IntentSummary,
	}
	for raw, want := range cases {
		if got := parseIntent(raw); got != want {
			t.Errorf("parseIntent(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestIntentClassifierInputIncludesRecentContext(t *testing.T) {
	history := []trpcmodel.Message{
		{Role: trpcmodel.RoleUser, Content: "第一步是怎么做的?"},
		{Role: trpcmodel.RoleAssistant, Content: "第一步是先构建检索索引。"},
	}
	got := intentClassifierInput("那第二个呢?", history)
	for _, want := range []string{
		"最近对话",
		"用户: 第一步是怎么做的?",
		"助手: 第一步是先构建检索索引。",
		"当前用户消息:\n那第二个呢?",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("intentClassifierInput missing %q in:\n%s", want, got)
		}
	}
}

func TestIntentClassifierInputTrimsHistory(t *testing.T) {
	long := strings.Repeat("很长", constant.IntentContextMessageMaxRunes)
	got := recentIntentContext([]trpcmodel.Message{{Role: trpcmodel.RoleUser, Content: long}})
	if len([]rune(got)) > constant.IntentContextMessageMaxRunes+len([]rune("用户: ")) {
		t.Fatalf("recentIntentContext 未截断, runes=%d", len([]rune(got)))
	}
}

func TestPaperResourceIntentOverride(t *testing.T) {
	boundCtx := core.WithPaperTitle(core.WithPaperID(context.Background(), "paper-1"), "Test Paper")
	cases := map[string]constant.IntentType{
		"有 github 仓库吗":               constant.IntentSummary,
		"有代码仓库吗":                     constant.IntentSummary,
		"这篇论文开源了吗":                   constant.IntentSummary,
		"project page 在哪":            constant.IntentSummary,
		"有没有 supplementary material": constant.IntentSummary,
		"作者提供数据集地址了吗":                constant.IntentSummary,
	}
	for q, want := range cases {
		got, ok := paperResourceIntentOverride(boundCtx, q)
		if !ok {
			t.Fatalf("paperResourceIntentOverride(%q) 未触发", q)
		}
		if got != want {
			t.Fatalf("paperResourceIntentOverride(%q) = %q, want %q", q, got, want)
		}
	}
}

func TestPaperResourceIntentOverrideDoesNotStealChitchat(t *testing.T) {
	boundCtx := core.WithPaperTitle(core.WithPaperID(context.Background(), "paper-1"), "Test Paper")
	cases := []struct {
		name  string
		ctx   context.Context
		query string
	}{
		{name: "unbound assistant account", ctx: context.Background(), query: "你有 github 账号吗"},
		{name: "bound assistant account", ctx: boundCtx, query: "你有 github 账号吗"},
		{name: "bound casual github", ctx: boundCtx, query: "github 真好用"},
		{name: "bound normal method", ctx: boundCtx, query: "这个模型结构是怎样的"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := paperResourceIntentOverride(tc.ctx, tc.query); ok {
				t.Fatalf("paperResourceIntentOverride(%q) = %q, want no override", tc.query, got)
			}
		})
	}
}

// TestWithBoundPaper 验证 ctx 带论文标题时导语拼在 prompt 前,未绑定时原样返回。
func TestWithBoundPaper(t *testing.T) {
	base := constant.SummaryAgenticPrompt
	if got := withBoundPaper(context.Background(), base); got != base {
		t.Errorf("未绑定论文时 prompt 被改写")
	}
	ctx := core.WithPaperTitle(context.Background(), "Agent Security Bench")
	got := withBoundPaper(ctx, base)
	if !strings.HasPrefix(got, "当前会话已绑定论文《Agent Security Bench》") {
		t.Errorf("导语未注入论文标题: %q", got[:min(len(got), 60)])
	}
	if !strings.HasSuffix(got, base) {
		t.Errorf("原 prompt 未完整保留在导语之后")
	}
}

// TestPolicyFor 验证意图到 agentic 工具迭代预算的映射:method 放宽,其余按概括预算。
func TestPolicyFor(t *testing.T) {
	if got := policyFor(constant.IntentMethod).MaxIter; got != constant.AgenticMaxIterMethod {
		t.Errorf("method MaxIter = %d, want %d", got, constant.AgenticMaxIterMethod)
	}
	if got := policyFor(constant.IntentSummary).MaxIter; got != constant.AgenticMaxIterSummary {
		t.Errorf("summary MaxIter = %d, want %d", got, constant.AgenticMaxIterSummary)
	}
	// 未知子类回退到概括预算。
	if got := policyFor(constant.IntentType("unknown")).MaxIter; got != constant.AgenticMaxIterSummary {
		t.Errorf("unknown MaxIter = %d, want %d", got, constant.AgenticMaxIterSummary)
	}
}
