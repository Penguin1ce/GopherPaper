package factory

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"GopherCPP/internal/config"
	"GopherCPP/pkg/constant"
)

// configPath 测试相对仓库根的配置路径。
const configPath = "../../config/config.toml"

// loadCfg 加载测试配置，缺失时跳过。
func loadCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Skipf("跳过：读取配置失败 %v", err)
	}
	return cfg
}

// loadFactory 加载配置并构造工厂，要求意图模型为 ollama，否则跳过。
func loadFactory(t *testing.T) *ModelFactory {
	t.Helper()
	cfg := loadCfg(t)
	if cfg.Models.Intent.Provider != constant.ProviderOllama {
		t.Skipf("跳过：意图模型 provider 非 ollama，当前 %q", cfg.Models.Intent.Provider)
	}
	return NewModelFactory(cfg)
}

// TestNewIntentModel_FromFactory 验证能从工厂拿到 ollama 意图模型。
func TestNewIntentModel_FromFactory(t *testing.T) {
	f := loadFactory(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m, err := f.NewIntentModel(ctx)
	if err != nil {
		t.Fatalf("创建意图模型失败 %v", err)
	}
	if m == nil {
		t.Fatal("意图模型为 nil")
	}
}

// TestNewChatModel_API 验证能从工厂拿到走 API 的下游大模型并真实可用。
// 需配置有效的 chat 模型与网络，缺配置或调用失败则跳过。
func TestNewChatModel_API(t *testing.T) {
	cfg := loadCfg(t)
	if cfg.Models.Chat.Provider == constant.ProviderOllama {
		t.Skipf("跳过：chat 模型为本地 ollama，非 API 模型")
	}
	if cfg.Models.Chat.APIKey == "" {
		t.Skip("跳过：未配置 chat 模型 api_key")
	}
	f := NewModelFactory(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	m, err := f.NewChatModel(ctx)
	if err != nil {
		t.Fatalf("创建 API chat 模型失败 %v", err)
	}
	if m == nil {
		t.Fatal("chat 模型为 nil")
	}

	out, err := m.Generate(ctx, []*schema.Message{
		schema.UserMessage("我是用户测试，你只需要说芝麻开门即可"),
	})
	if err != nil {
		t.Skipf("跳过：调用 API 模型失败，确认网关与密钥可用 %v", err)
	}
	if out == nil || out.Content == "" {
		t.Fatal("API 模型返回内容为空")
	}
	t.Logf("API 模型响应 %q", out.Content)
}
