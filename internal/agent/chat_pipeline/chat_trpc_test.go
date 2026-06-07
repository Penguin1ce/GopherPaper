package chat_pipeline

import (
	"context"
	"strings"
	"testing"
	"time"

	"GopherPaper/internal/config"
	"GopherPaper/internal/factory"
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

// TestClassifyIntentTRPC 验证意图分类链路:返回值须是三个合法子类之一。
func TestClassifyIntentTRPC(t *testing.T) {
	cfg, _ := loadChatTestModels(t)
	m := factory.NewTRPCChatModel(cfg.Models.Intent)

	cases := map[string]constant.IntentType{
		"这篇论文在 DBLP 数据集上的准确率是多少？": constant.IntentFact,
		"帮我概括一下这篇论文讲了什么":          constant.IntentSummary,
		"它用了什么模型结构和实验设计？":         constant.IntentMethod,
	}
	valid := map[constant.IntentType]bool{constant.IntentFact: true, constant.IntentSummary: true, constant.IntentMethod: true}

	for q, want := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		got := ClassifyIntentTRPC(ctx, m, cfg.Models.Intent, q)
		cancel()
		if !valid[got] {
			t.Fatalf("分类返回非法子类: %q -> %q", q, got)
		}
		t.Logf("分类 %q -> %q (期望参考 %q)", q, got, want)
	}
}

// TestChatTRPC 验证 chat 切片端到端:分类 + RAG 生成 + 意图标记。
// 检索器未 Init 时降级为无片段(像 report 切片),仍能生成,故不依赖 Milvus。
func TestChatTRPC(t *testing.T) {
	cfg, _ := loadChatTestModels(t)
	intentModel := factory.NewTRPCChatModel(cfg.Models.Intent)
	chatModel := factory.NewTRPCChatModel(cfg.Models.Chat)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ctx = tenant.With(ctx, tenant.Tenant{StudentID: "test-user"})

	reply, err := ChatTRPC(ctx, intentModel, chatModel, cfg.Models.Intent, cfg.Models.Chat, nil, "请简要介绍这篇论文的研究方法")
	if err != nil {
		t.Fatalf("ChatTRPC 失败: %v", err)
	}
	if strings.TrimSpace(reply.Content) == "" {
		t.Fatal("回答内容为空")
	}
	if reply.Intent != constant.IntentFact && reply.Intent != constant.IntentSummary && reply.Intent != constant.IntentMethod {
		t.Fatalf("回答意图非法: %q", reply.Intent)
	}
	t.Logf("trpc chat 切片跑通: intent=%q 内容长度=%d", reply.Intent, len(reply.Content))
}
