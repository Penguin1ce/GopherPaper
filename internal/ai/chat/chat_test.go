package chat

import (
	"context"
	"strings"
	"testing"
	"time"

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
		"这篇论文在 DBLP 数据集上的准确率是多少？": constant.IntentFact,
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
	if reply.Intent != constant.IntentFact && reply.Intent != constant.IntentSummary && reply.Intent != constant.IntentMethod {
		t.Fatalf("回答意图非法: %q", reply.Intent)
	}
	t.Logf("trpc chat 切片跑通: intent=%q 内容长度=%d", reply.Intent, len(reply.Content))
}
